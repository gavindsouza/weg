/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package remote

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/remote"
	"github.com/spf13/cobra"
)

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull changes from the remote site",
	Long: `Fetch changes from the remote site and update local files.

By default, pull reconstructs the site's version history since the last sync:
it fetches the Version records newer than the recorded last_sync and replays
them as individual, authored git commits (the same style as clone), so
'git log' and 'git blame' stay meaningful. Each replayed commit holds the whole
document: replay starts from the committed file, and a document's last change
in the window is committed as the site has it now. Custom Field and Property
Setter changes update their row in the per-DocType file, and a document
created in the window is added in a commit by its creator. Any current-state
changes that have no Version record land in a final "chore(sync): reconcile"
commit, so nothing is dropped.

Frappe writes no Version on delete, so deletions are replayed from the site's
Deleted Document rows: each deletion of a document the export holds becomes a
commit by the user who deleted it, removing its file (or its row from the
per-DocType Custom Field / Property Setter file).

A list can come back short, so a committed file (or a Custom Field /
Property Setter row) the fetch does not produce goes only when the site,
asked for that record by name, confirms it:
  - it answers 404 (not found), and the file held what the site had: the
    site recorded the deletion in a Deleted Document row inside the window
    (it goes in a commit by who deleted it, dated when), or the push
    baseline (.weg/last_push_commit) shows the file has no local work (it
    goes in the reconcile commit). Without a baseline (a CI checkout) and
    with no deletion recorded in the window, the file may be local work not
    pushed yet, so it stays, with a warning;
  - the fetch exports the record at another path and the site's copy places
    it there (a module move): the old file goes in the reconcile commit;
  - the site returns the record but would not export it (a DocType that is
    no longer custom, a Report or Print Format made standard, a
    system-generated Custom Field): it goes in the reconcile commit.
Anything else keeps the file, with a warning: any other answer (403, 429,
5xx, a timeout), a record the site still has and would export, a move it
does not confirm. A Deleted Document row is replayed only when the site
confirms its record is gone (or exports it again), so a document restored
since is never removed. Files with local work not yet pushed always stay,
nothing goes when a DocType's module could not be read, in modules marked
sync = false only deletions recorded in the window remove files, and files
of entity types disabled in [sync.entities] are never removed. A snapshot
pull (--no-history) removes the same files, all in its one commit.

Frappe records times as wall-clock time in the site's time zone (System
Settings, or Asia/Kolkata when unset), so commits carry that zone's UTC offset.
If System Settings cannot be read (the API user needs read access on it), pull
stops before replaying anything and exits non-zero, leaving last_sync as it
was: the version window cannot be placed without the zone.

If the version history cannot be fetched, pull still commits the current state
as a single snapshot so the working tree is usable, prints warnings tagged
"history-fallback:" and exits non-zero, so scheduled jobs notice that the
window was recorded without per-change history. If the replay fails or is
interrupted (Ctrl-C), pull restores the export to the commit it started from,
keeping no replayed commits and leaving last_sync as it was, so a rerun
replays the same window cleanly.

Pass --no-history for the fast path: a single snapshot commit of the current
state (the pre-history behavior).

This command:
  1. Connects to the remote site
  2. Fetches all enabled entity types
  3. Replays version history since last_sync as per-document commits
     (or a single snapshot commit with --no-history)
  4. Updates last_sync

Examples:
  weg remote pull                    # Pull, reconstructing history since last sync
  weg remote pull --no-history       # Pull as a single snapshot commit
  weg remote pull -m "Update scripts" # Snapshot pull with a custom message (implies --no-history)
  weg remote pull --dry-run          # Preview changes without applying`,
	RunE: runPull,
}

var (
	pullDryRun    bool
	pullMessage   string
	pullNoHistory bool
)

func init() {
	pullCmd.Flags().BoolVar(&pullDryRun, "dry-run", false, "Preview changes without applying")
	pullCmd.Flags().StringVarP(&pullMessage, "message", "m", "", "Commit message for a snapshot pull (implies --no-history)")
	pullCmd.Flags().BoolVar(&pullNoHistory, "no-history", false, "Skip version history (single snapshot commit)")
}

func runPull(cobraCmd *cobra.Command, args []string) error {
	// Check if we're in a remote site directory
	if !remote.IsRemoteSite(".") {
		return wegerrors.NotFound("remote clone", ".weg/site.toml")
	}

	// Load config and credentials
	config, err := remote.LoadSiteConfig(".")
	if err != nil {
		return wegerrors.Config("site.toml", "read", err)
	}

	creds, err := remote.LoadCredentials(".")
	if err != nil {
		return wegerrors.Config("credentials", "read", err)
	}

	// Connect
	output.Infof("Connecting to %s...\n", config.Site.URL)
	client := remote.NewClientFromConfig(config, creds)
	if err := client.Ping(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	output.Print("Connected")

	// Fetch entities. The sync time is taken before the fetch, so a site
	// change made while it runs is picked up again by the next pull rather
	// than missed; Versions newer than it are left to the next pull too.
	output.Print("Fetching customizations...")
	syncedAt := time.Now()
	fetcher := remote.NewFetcher(client, config)
	result, err := fetcher.FetchAll()
	if err != nil {
		return fmt.Errorf("failed to fetch: %w", err)
	}

	if pullDryRun {
		output.Printf("\nDry run - would update %d entities:", len(result.Entities))
		for _, e := range result.Entities {
			output.Printf("  %s: %s", e.Type, e.Name)
		}
		committed, err := readCommittedFiles(".")
		if err != nil {
			return err
		}
		plan := planStaleExport(committed, result, fetcher, readLocalState("."), nil)
		if len(plan.Removals) > 0 {
			output.Printf("\nWould remove %d files the site no longer has:", len(plan.Removals))
			for _, r := range plan.Removals {
				output.Printf("  - %s (%s)", r.Path, r.Reason)
			}
		}
		return nil
	}

	// A custom commit message only makes sense for a single snapshot commit.
	startHead, clean := gitHead("."), exportClean(".")
	snapshotOnly := pullNoHistory || pullMessage != ""
	if snapshotOnly {
		err = pullSnapshot(config, fetcher, result, syncedAt)
	} else {
		err = pullWithHistory(cobraCmd.Context(), config, fetcher, result, syncedAt)
	}
	advancePushBaseline(".", startHead, clean)
	return err
}

// exportClean reports whether the export has no uncommitted changes.
func exportClean(dir string) bool {
	out, err := gitCmd(dir, "status", "--porcelain", "--", ".").Output()
	return err == nil && len(out) == 0
}

// advancePushBaseline moves the push baseline (.weg/last_push_commit) over
// the commits a pull made, when it pointed at the commit the pull started
// from and the export had no uncommitted changes (which the pull's final
// commit would have taken in): those commits then hold only site state, and
// a push must not send them back, least of all a removed file as a deletion
// of the document on the site. Otherwise it stays, and any file the pull
// removed is named, since the next push lists it as a deletion.
func advancePushBaseline(dir, startHead string, clean bool) {
	head := gitHead(dir)
	if startHead == "" || head == "" || head == startHead {
		return
	}
	path := filepath.Join(dir, ".weg", "last_push_commit")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	if clean && strings.TrimSpace(string(data)) == startHead {
		if err := os.WriteFile(path, []byte(head), 0o644); err != nil {
			output.Warningf("Could not update the push baseline: %v", err)
		}
		return
	}
	out, err := gitCmd(dir, "diff", "--name-only", "-z", "--relative", "--diff-filter=D", startHead, head, "--", ".").Output()
	if err != nil {
		return
	}
	var removed []string
	for _, p := range strings.Split(string(out), "\x00") {
		if p != "" {
			removed = append(removed, p)
		}
	}
	if len(removed) > 0 {
		output.Warningf("This pull removed %d files the site no longer has, but local changes are unpushed, so 'weg remote push' will list them as deletions; skip those when it asks:", len(removed))
		for _, p := range removed {
			output.Warningf("  - %s", p)
		}
	}
}

// pullSnapshot writes the current entity state and records it as a single git
// commit (the --no-history path, and the pre-history default behavior).
func pullSnapshot(config *remote.SiteConfig, fetcher *remote.Fetcher, result *remote.FetchResult, syncedAt time.Time) error {
	local := readLocalState(".")
	committed, err := readCommittedFiles(".")
	if err != nil {
		return err
	}
	// Drop only what the site confirms is gone, so a snapshot converges too;
	// rows the site still has are kept in their files.
	plan := planStaleExport(committed, result, fetcher, local, nil)
	// Write entities
	output.Infof("Updating %d entities...\n", len(result.Entities))
	for _, entity := range result.Entities {
		if err := remote.WriteEntity(".", entity); err != nil {
			output.Errorf("Failed to write %s: %v", entity.Name, err)
		}
	}
	if err := removeExportFiles(".", plan.Removals, nil); err != nil {
		return err
	}

	// Keep the workspace in step with the freshly-written JSONs
	refreshWorkspaceAfterPull(".")

	// Update config
	config.Sync.LastSync = syncedAt
	if err := config.Save("."); err != nil {
		return wegerrors.Config("site.toml", "write", err)
	}

	// Check for changes and commit. Only the export is staged: in a
	// subdirectory (weg -C), the rest of the repository is not the site's.
	gitStatus := exec.Command("git", "status", "--porcelain", "--", ".")
	statusOutput, _ := gitStatus.Output()
	if len(statusOutput) == 0 {
		output.Print("Already up to date")
		return nil
	}

	gitAdd := exec.Command("git", "add", "-A", "--", ".")
	if err := gitAdd.Run(); err != nil {
		return fmt.Errorf("failed to stage changes: %w", err)
	}

	commitMsg := pullMessage
	if commitMsg == "" {
		commitMsg = fmt.Sprintf("Pull from %s at %s",
			config.Site.URL, time.Now().Format("2006-01-02 15:04"))
	}
	gitCommit := exec.Command("git", "commit", "-m", commitMsg, "--", ".")
	if out, err := gitCommit.CombinedOutput(); err != nil {
		return wegerrors.Operation("git commit", strings.TrimSpace(string(out)), err)
	}

	output.Print("Changes committed")
	return nil
}

// pullWithHistory replays Version records created since last_sync as individual
// authored commits, starting each document from its committed file, then
// reconciles the current state into a final snapshot commit. last_sync is
// written only right before that commit (and lands in it), so a failed pull
// re-fetches the same window on the next run.
func pullWithHistory(ctx context.Context, config *remote.SiteConfig, fetcher *remote.Fetcher, result *remote.FetchResult, syncedAt time.Time) error {
	return reconstructHistory(ctx, historyRun{
		dir:           ".",
		siteURL:       config.Site.URL,
		frappeVersion: config.Site.Frappe.Version,
		modulesFile:   "modules.txt",
		fetcher:       fetcher,
		result:        result,
		since:         config.Sync.LastSync,
		until:         syncedAt,
		fromCommitted: true,
		recordSync: func() (func(), error) {
			prev := config.Sync.LastSync
			config.Sync.LastSync = syncedAt
			if err := config.Save("."); err != nil {
				config.Sync.LastSync = prev
				return nil, wegerrors.Config("site.toml", "write", err)
			}
			return func() {
				config.Sync.LastSync = prev
				if err := config.Save("."); err != nil {
					output.Warningf("Could not restore last_sync in site.toml: %v", err)
				}
			}, nil
		},
	})
}
