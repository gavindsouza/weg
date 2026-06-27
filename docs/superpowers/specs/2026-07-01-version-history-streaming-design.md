# Streaming, resumable version-history reconstruction

**Date:** 2026-07-01
**Status:** Approved
**Area:** `weg remote clone` — version history / git reconstruction

## Problem

`weg remote clone --history` (the default) reconstructs git history from a Frappe
site's `Version` records. The original implementation:

1. Fetched **all** `Version` rows for the tracked doctypes into memory in one
   sequential paginated pass (`GetAllVersions`), each row carrying its full `data`
   JSON blob.
2. Built the entire commit plan in memory (`BuildCommitPlan`), reconstructing
   every historical file state **backward from current state**, before writing
   anything to disk.

Consequences on a real multi-year ERPNext site:

- **Slow / looks hung** — one `print("Fetching version history...")` covers a
  multi-minute sequential fetch of a bloated `Version` table with no progress.
- **Memory** — peak holds all raw version blobs + a duplicated `HistoryEntry`
  copy + every reconstructed file state simultaneously (multi-GB risk).
- **Fragile** — a single 30s client timeout aborted the whole fetch, silently
  dropping history (fixed separately: 120s default + `WEG_HTTP_TIMEOUT` +
  retry-on-timeout). Nothing is resumable: a failure loses all fetched work.
- **Cannot represent deleted docs** — backward reconstruction needs a current
  state to walk back from, so deleted entities have no anchor.

## Requirements (agreed)

- Preserve **full** history, including **deleted** and **renamed** entities
  (goal: GitLens-style per-file blame in `weg_workspace`).
- **Bounded memory** — no multi-GB spikes regardless of `Version` table size.
- **Resumable** — a crash/timeout mid-run must not require refetching everything.
- **Fast** — parallelize the fetch.
- Per-user divergence (some want history, some don't) stays served by the
  existing `--no-history` fast path; history remains the default.

## Design

Disk-staged streaming pipeline, stdlib only (no new deps).

### Phase 1 — Fetch (parallel, streaming to disk, resumable)

New `internal/remote/versionfetch.go`.

- One goroutine per versioned doctype (10; `sync.WaitGroup`, no worker pool at
  that count). First error cancels the rest via `context`.
- Each paginates `Version` filtered `ref_doctype == DT`,
  `order_by=creation asc, name asc`, `limit_page_length=100`, appending each row
  as one JSON line to `.weg/tmp/versions/<doctype>.jsonl`.
- After each flushed page, write `.weg/tmp/versions/<doctype>.cursor` = **count
  of records written so far**. Cursor is written *after* the data is flushed, so
  the JSONL is never behind the cursor.
- Memory = one page (~100 rows) per doctype; nothing accumulates.
- Reuses the existing timeout-retry in `client.request`.
- **[ceiling]** offset pagination on a live table can skip/dup a row if the site
  mutates during the clone. Acceptable for a one-shot snapshot; upgrade to keyset
  pagination only if it proves a problem.

### Phase 2 — Resolve deleted custom DocTypes

- Diff DocType docnames seen in the JSONL against the current `custom=1` set from
  `FetchAll`.
- For unknowns, one **batched** query to Frappe's `Deleted Document` trash table
  → read each deleted doc's stored `custom` flag → keep only confirmed-custom.
- Other tracked doctypes (Custom Field, Property Setter, Client/Server Script,
  Report, Print Format, Workflow, Notification, Letter Head) are customizations
  by nature — all their history is in scope, no confirmation needed.
- **[ceiling]** if `Deleted Document` is disabled/pruned, unprovable names are
  skipped (logged), not guessed.

### Phase 3 — Build (streaming, forward reconstruction)

Reworks `BuildCommitPlan` into a streaming builder.

- **k-way merge:** min-heap over the 10 JSONL stream heads, emitting the globally
  earliest record by `(creation, name)`. Memory = 10 buffered rows + heap.
- Maintain `liveState[filePath]` — the reconstructed current state per entity.
  Memory = **O(entities)**, not O(versions). The bloat streams past on disk;
  only the small set of entity states stays resident.
- Per version, in chronological order: forward-apply the diff
  (`changed` → new value, `added` → add child row, `removed` → drop child row),
  write the file, `git commit --author=<owner> --date=<creation>`. Owner names
  resolved via the existing user lookup.
- **Deleted** entity (confirmed) → after its last version, a final commit
  *removing* the file. History lives in git; the file is not at HEAD.
- **Renamed** entity → versions under both docnames both reconstruct, so both
  histories are preserved. **[ceiling]** not stitched via `git mv`; the stale old
  path is cleaned in Phase 4. True rename-follow is a later refinement.

### Phase 4 — Reconcile to current

- For entities that still exist: if forward-reconstructed `liveState` diverges
  from the current fetched doc (un-versioned initial fields, gaps), one final
  `Weg`-authored "sync to current" commit. Guarantees **HEAD == current site
  state** regardless of version-log completeness.
- Any HEAD file not in the current set and not a confirmed deletion (e.g. a
  renamed-away old path) is removed here.

### Resume model

The fetch is the slow/flaky part; the build is local and fast. Therefore:

- `.weg/tmp/versions/*.jsonl` + `.cursor` are the durable, expensive artifact.
- Re-running `weg remote clone` into a dir containing `.weg/tmp/` → **resume
  fetch** from cursors (truncating any JSONL beyond the cursor count first, to
  stay consistent), then **rebuild git history from scratch** off the JSONL.
- Build is **not** checkpointed: if interrupted, reset and rebuild from the
  cached JSONL (cheap). No build-phase bookkeeping.
- `.weg/tmp/` is gitignored, deleted on success, kept on failure.

## Error handling

- Fetch page timeout → existing retry; persistent failure preserves the cursor
  for resume.
- Crash mid-page-write → JSONL may hold an incomplete trailing line ahead of the
  cursor; on resume, truncate JSONL to `cursor` complete lines before continuing.
- `Deleted Document` unavailable → skip unprovable deleted DocTypes, logged.
- Build is deterministic; on failure, rerun rebuilds from JSONL.

## Testing (pure, no network)

- `build_test.go`: synthetic diff sequence (create → change → add child →
  remove child → delete) → assert file state at each step + final removal.
- k-way merge ordering across a couple of JSONL fixtures.
- resume truncation: cursor=N truncates JSONL to N complete lines.

## Explicitly out of scope (YAGNI)

- SQLite staging (escape hatch only, if JSONL merge proves too slow).
- git-mv rename-follow (renamed entities keep both histories for now).
- Build-phase checkpointing (rebuild from cached JSONL instead).
