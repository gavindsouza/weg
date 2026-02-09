# Expert Panel 2: API Design & Testing Strategy Review

**Date:** 2026-02-09
**Reviewers:** Sebastian Ramirez (simulated), Guido van Rossum (simulated), Mitchell Hashimoto (simulated)
**Project:** weg — Go CLI for Frappe development
**Scope:** API surface design, documentation quality, testing infrastructure, configuration system

---

## Facts Used in This Review

- **FACT-F1:** weg is a Go 1.24 CLI for Frappe development (replacement for `bench` CLI)
- **FACT-F2:** 222 Go files, ~35k LOC code + ~10k LOC tests, 70+ commands across 20 groups
- **FACT-F3:** Uses cobra for CLI, 7 direct dependencies only
- **FACT-F4:** Three modes: app-centric (pyproject.toml), bench-centric (weg.toml), remote-site
- **FACT-F5:** Custom error types with exit codes (8 types), proper error wrapping
- **FACT-F6:** State management via .weg/state.json with file locking (syscall.Flock) and atomic writes
- **FACT-F7:** MCP server for AI integration (12 tools, subprocess + in-process handlers)
- **FACT-F8:** Output system supports json/table/plain/quiet with automatic secret redaction
- **FACT-F9:** CI/CD with gofmt, go vet, race detector tests, multi-platform release
- **FACT-F10:** 1 GitHub star, no releases yet, no issues, sole maintainer
- **FACT-F13:** Test coverage: internal/errors 92.9%, internal/completion 84.4%, but cmd/site 4.5%, cmd/app 7.4%
- **FACT-F14:** Has PRODUCT_ROADMAP.md, USAGE.md, docs/CLI_CONVENTIONS.md
- **FACT-F15:** Shell completion support exists in internal/completion (84.4% coverage)

---

## Sebastian Ramirez's Review

### Summary

*"I've seen a lot of CLI tools that ship with 70 commands and zero discoverability. Weg has clearly been built by someone who thinks about command structure, and the help text is leagues ahead of most CLI tools I see. But the documentation is uneven — the README is excellent, the in-command help ranges from great to bare minimum, and there's a huge gap between 'we have docs' and 'the docs teach you everything you need to know.' If you can't discover how to use a feature from the help text alone, the tool has a documentation bug."*

### Findings

**SR-1: Excellent README with working examples — SEVERITY: Positive**

The README (`README.md:1-344`) is genuinely well-structured. It opens with three concrete workflows, shows quick-start examples for each mode, and includes a comparison table against bench. The "Three Development Modes" section with actual `bash` snippets at lines 9-48 is exactly how I'd want to discover a tool. The "Common Commands" reference at lines 107-202 is a practical cheat-sheet.

This is above average for a pre-release project. The README tells a story.

**SR-2: API command help text is a model for other commands — SEVERITY: Positive**

`cmd/api/api.go:27-51` — The `ApiCmd.Long` description is genuinely excellent. It explains local vs remote mode, documents the credential resolution hierarchy (CLI flags → env → config file), and provides examples for both modes. Similarly, `cmd/api/get.go:23-34` shows the `getCmd` help with multiple practical examples including filters, field selection, and ordering.

This is the gold standard that every other command in this project should aspire to.

**SR-3: Many commands have minimal or no Long description — SEVERITY: High**

`cmd/site/site.go:11-17` — The `SiteCmd` has a Long description with only 4 example lines. Contrast this with `ApiCmd` which explains two modes, credential resolution, and provides 8 example lines. Many subcommands likely have only `Short` descriptions.

The site commands are arguably the most-used commands in weg (new users will `site new`, `site list`, `site drop` constantly), yet they have the least help text. Compare:
- `api` parent: 15 lines of Long, explains modes, credential hierarchy, 8 examples
- `site` parent: 4 lines of Long, just re-lists subcommand names that `--help` already shows

**Recommendation:** Every parent command should explain its conceptual model. `site` should explain that sites are per-bench, what "default site" means, and how sites relate to apps.

**SR-4: Output format system is auto-documenting — SEVERITY: Positive**

`internal/output/output.go:20-28` — The `FormatAuto` that detects TTY vs pipe and switches between table/JSON is exactly right. `cmd/root.go:154` exposes this as `--output auto, json, table, plain, quiet`. The auto-detection at `output.go:167-175` means scripting Just Works — pipe to `jq` and you get JSON, run in terminal and you get tables.

This is FastAPI-level "it just works" design. Very impressed.

**SR-5: Shell completion exists but isn't discoverable from help — SEVERITY: Medium**

`internal/completion/completion.go` provides `CompleteSiteNames`, `CompleteAppNames`, `CompleteDocTypes` with smart filtering. The README documents completion setup at lines 288-310. But the `weg --help` output doesn't mention completions, and there's no `weg completion --help` that explains *what* gets completed.

Users running `weg <tab>` will get subcommand completion from cobra, but won't know that `weg api get <tab>` completes DocType names or that `weg site use <tab>` completes site names unless they try it.

**Recommendation:** Add a note in `completion` subcommand help: "Weg provides intelligent completions for site names, app names, and DocType names in addition to subcommands."

**SR-6: CLI_CONVENTIONS.md is excellent but users can't find it — SEVERITY: Medium**

`docs/CLI_CONVENTIONS.md` defines flag naming, output formats, exit codes, error messages, confirmation prompts, and status symbols. This is exactly the kind of spec document I'd write for a team. But it's buried in `docs/` and the README doesn't link to it.

More importantly, this document is for *contributors*, not *users*. There's no user-facing guide to "how weg output works" that explains `--output`, `-v`/`-vv`/`-vvv`, `--debug-categories`, and secret redaction in one place.

**SR-7: No `--help` examples for destructive commands — SEVERITY: Medium**

`cmd/site/site.go:20-26` — The `dropCmd`, `installCmd`, etc. are registered but based on the test file (`cmd/site/site_test.go:62-90`), the tests only verify that `RunE` is set, not that help text contains examples. For destructive commands like `site drop`, the help should show:
```
Examples:
  weg site drop mysite.localhost          # Interactive confirmation
  weg site drop mysite.localhost --force  # Skip confirmation
  weg -y site drop mysite.localhost       # Global yes flag
```

### Recommendations

1. **Documentation parity audit**: Every parent command should match `api`'s help quality. Priority: `site`, `app`, `remote`, `docker`, `cloud`.
2. **Add `Examples:` section to every command's Long description**: Cobra renders these beautifully. Use `cmd.Example` field for even better formatting.
3. **Create a user-facing "Output & Debugging" guide**: Combine the verbosity table from CLI_CONVENTIONS.md with the redaction behavior and format options.
4. **Add `--help` integration tests**: Verify that help text for key commands contains `Examples:` and essential concepts.
5. **Link CLI_CONVENTIONS.md from CONTRIBUTING.md** (when created) so new contributors follow the patterns.

---

## Guido van Rossum's Review

### Summary

*"The three-mode design is ambitious and I can see why it's attractive, but I worry about the implicit behavior explosion. When a user runs `weg start`, what happens depends on which of 5 contexts the directory detection returns — and the user may not know which one they're in. The Go code is readable and the naming is mostly consistent, but there are places where implicit fallback chains make it hard to predict behavior. 'Explicit is better than implicit' isn't just about Python — it's about every tool that touches a user's development environment."*

### Findings

**GVR-1: The five-context detection is implicit and potentially confusing — SEVERITY: High**

`internal/config/detect.go:52-116` — `DetectContext` returns one of five contexts: `ContextFresh`, `ContextApp`, `ContextBench`, `ContextWegApp`, `ContextWegBench`. The detection priority is:

1. `weg.toml` → `ContextWegBench`
2. `.weg/` directory + `hooks.py` → `ContextWegApp`
3. `.weg/` directory alone → `ContextWegBench`
4. `pyproject.toml` with `[tool.weg]` → `ContextWegApp`
5. `hooks.py` in dir or subdir → `ContextApp`
6. `apps/` + `sites/` → `ContextBench`
7. Nothing → `ContextFresh`

This means a user who clones a Frappe app (has `hooks.py`) and runs `weg init` transitions from `ContextApp` → `ContextWegApp`. But if they accidentally have both `weg.toml` AND `pyproject.toml [tool.weg]`, the detection silently picks `ContextWegBench` (weg.toml wins at line 64-69). There's no warning.

**Recommendation:** Add a `weg status` or `weg context` command that explicitly shows what weg detected and why. When ambiguous signals exist (e.g., both `weg.toml` and `pyproject.toml [tool.weg]`), print a warning.

**GVR-2: The `api` command has two completely different code paths with no obvious indicator — SEVERITY: High**

`cmd/api/get.go:47-119` — The `runGet` function has two entirely different execution paths: local mode (Python subprocess, lines 100-118) and remote mode (HTTP API, lines 78-97). The mode is determined by `isRemoteMode()` which checks if `--url` is set (`api.go:134-136`).

The problem: the user sees the same `weg api get User` command in both cases, but the behavior, authentication, error modes, and data formats may differ. Local mode runs as Administrator with no HTTP; remote mode uses API keys with HTTP. If a user is inside a remote-site clone directory, should `weg api get` work against the remote site automatically? Currently it doesn't — it only uses `--url`.

This is a case where "There should be one obvious way to do it" conflicts with the desire for a unified interface. The `api` help text at `api.go:27-51` does explain both modes clearly, which helps. But the implicit mode switching based on a single flag is a trap.

**GVR-3: Naming inconsistency between `weg api` and `weg doc` — SEVERITY: Medium**

From the README (`README.md:157-163`):
```
weg api call frappe.client.get_count doctype=User
weg doc get User Administrator   # Get a document
weg doc list User --limit 10     # List documents
```

Both `weg api get User/Administrator` and `weg doc get User Administrator` retrieve the same document. Having two ways to do the same thing violates the "one obvious way" principle. The difference seems to be:
- `weg api` is REST-oriented (doctype/name format, supports filters/fields)
- `weg doc` is document-oriented (separate positional args)

But why would a user choose one over the other? This needs clarification or one of them should be deprecated.

**GVR-4: The `skipAutoChdir` map is a maintenance hazard — SEVERITY: Medium**

`cmd/root.go:97-109` — The `PersistentPreRunE` uses a hardcoded map of command names that should skip auto-directory-detection:

```go
skipAutoChdir := map[string]bool{
    "new":        true,
    "create":     true,
    "init":       true,
    "help":       true,
    "version":    true,
    "completion": true,
    "self":       true,
    "run":        true,
    "clone":      true,
    "remote":     true,
    "workspace":  true,
}
```

Every new command that should work outside a project must be manually added here. This is implicit — a developer adding a new top-level command won't know about this list, and their command will silently fail when run outside a project directory.

**Recommendation:** Use cobra annotations or a command-level flag/field to mark commands as "works outside project" rather than a centralized list. Or invert the logic: default to "no auto-chdir" and explicitly mark commands that *require* a project.

**GVR-5: Global mutable state in `output` package — SEVERITY: Medium**

`internal/output/output.go:55-77` — The output package uses package-level mutable globals:
```go
var (
    CurrentFormat Format = FormatAuto
    Level Verbosity = VerbosityNormal
    DebugCategories map[DebugCategory]bool
    NoColor bool
    Writer io.Writer = os.Stdout
    ErrWriter io.Writer = os.Stderr
)
```

Every test that exercises output must save/restore these globals (seen extensively in `internal/output/output_test.go` — the pattern `oldWriter := Writer; defer func() { Writer = oldWriter }()` appears 20+ times). This is fragile and prevents parallel test execution on anything that touches output.

This is the kind of global state that seems convenient early but becomes a ball and chain. In Python, I'd call this "import-time side effects."

**GVR-6: The `run` subcommand in `api.go` has an unfortunate verb collision — SEVERITY: Low**

From USAGE.md line 204-208:
```
weg api run "Sales Invoice/INV-001" submit
weg api run "Sales Invoice/INV-001" cancel
```

The verb "run" collides conceptually with `weg run` (which "clones fresh" per root.go:105). Using "run" to mean "execute a document method" is non-obvious. Better: `weg api invoke` or `weg api exec`.

**GVR-7: Context detection walks up the directory tree — subtle behavior — SEVERITY: Low**

`internal/config/detect.go:119-153` — `FindBenchRoot` walks up the directory tree. This means running `weg site list` from `~/projects/myapp/myapp/doctype/` will auto-chdir to `~/projects/myapp/` if it has `.weg/`. This is mostly good (matches git's behavior), but combined with 5 context types, it can produce surprising results if nested projects exist.

### Recommendations

1. **Add `weg context` or enhance `weg status`** to explicitly show: detected context, config file path, bench path, default site, and how credentials will be resolved. This single command would demystify the implicit detection.
2. **Clarify `weg api` vs `weg doc` relationship** in help text, or deprecate one in favor of the other. If both stay, document when to use which.
3. **Replace `skipAutoChdir` map** with a cobra annotation or per-command opt-in mechanism.
4. **Add ambiguity warnings** when multiple context signals exist (e.g., both weg.toml and pyproject.toml with [tool.weg]).
5. **Consider an `OutputConfig` struct** passed through context rather than global mutable state, at least for new code.

---

## Mitchell Hashimoto's Review

### Summary

*"The internal packages — state, errors, output, config, completion — are solid. I can see the bones of a well-structured Go CLI here. The error types with exit codes, the atomic file writes with locking, the state diffing — these are patterns I'd expect from an experienced Go developer. But the testing story has a massive gap: the cmd/ packages are essentially untested for actual behavior. The tests verify 'is the command registered' but never 'does the command produce the right output.' This is the difference between a project that works and a project I'd trust in production."*

### Findings

**MH-1: cmd/ tests are structural, not behavioral — SEVERITY: Critical**

`cmd/site/site_test.go:1-91` — Every test in this file follows the same pattern:

```go
func TestSiteCommand_Setup(t *testing.T) {
    if SiteCmd.Use != "site" { ... }
    if SiteCmd.Short != "Manage Frappe sites" { ... }
    subcommands := SiteCmd.Commands()
    // Verify subcommands are registered
}

func TestDropCommand_Setup(t *testing.T) {
    if dropCmd.Name() != "drop" { ... }
    if dropCmd.RunE == nil { ... }
}
```

These tests verify *registration*, not *behavior*. They're the equivalent of testing that a function exists, not that it returns the right value. At 4.5% coverage for `cmd/site`, you're testing the cmd struct literals, not the `RunE` implementations.

Similarly, `cmd/app/app_test.go:1-91` follows the identical pattern — structural verification only. The one exception is `TestParseAppSpec` (lines 93-143) and `TestExtractAppName` (lines 145-181), which are genuine unit tests of parsing logic. These are exactly the right pattern; the rest of the file is busywork.

**MH-2: No acceptance tests or golden file tests — SEVERITY: Critical**

The project has `tests/integration_test.go` which tests cross-package workflows (remote site setup, output formatting, workspace state, config detection). These are good but they test *internal APIs*, not *CLI behavior*.

What's missing is acceptance tests: "run `weg site list` in a test bench directory and verify the output matches expected format." HashiCorp projects like Terraform use a pattern:

```go
func TestSiteList_JSON(t *testing.T) {
    // Setup: create temp bench with known sites
    // Execute: run command, capture stdout/stderr
    // Verify: compare output to golden file or expected structure
}
```

The cobra framework supports `cmd.SetOut()` and `cmd.SetErr()` for capturing output. Combined with `cmd.SetArgs()`, you can test full command execution without subprocess overhead.

**MH-3: Test helper `setupTestBench` is good but needs to be shared — SEVERITY: High**

`internal/completion/completion_test.go:153-190` defines `setupTestBench` which creates a complete test bench directory with apps, sites, site_config.json files, and weg.toml. This is exactly the kind of test fixture that should be in a shared `internal/testutil` package.

Currently each package that needs a bench fixture will either duplicate this helper or skip testing entirely. A shared `testutil.NewBenchFixture(t)` with builder pattern options would dramatically lower the barrier to writing cmd/ tests:

```go
func TestSiteList(t *testing.T) {
    bench := testutil.NewBench(t).
        WithSite("test.localhost", "frappe", "erpnext").
        WithSite("prod.localhost", "frappe").
        Build()
    // Now test against bench.Path
}
```

**MH-4: State management is well-designed with proper locking — SEVERITY: Positive**

`internal/state/state.go:74-98` (Load) and `state.go:127-168` (Save) implement file locking with `syscall.Flock` and atomic writes (write-to-temp + rename). The graceful fallback to unlocked reads at lines 87-88 and 93-94 is pragmatic — don't fail if locking isn't supported. The `State.Validate()` method at lines 297-348 checks structural invariants (no duplicate defaults, no dangling app references).

This is textbook Go state management. The diff computation in `diff_test.go` with `ComputeDiffFromBenchConfig` testing add/remove/update scenarios for both apps and sites is thorough.

**MH-5: Error type hierarchy is clean and well-tested — SEVERITY: Positive**

`internal/errors/errors.go:1-336` defines 8 error types, each with proper `Error()`, `Unwrap()`, exit code mapping, and factory functions. The `IsUserError()` and `IsRetryable()` classification functions are exactly what you need for a CLI. The test file at `internal/errors/errors_test.go:1-367` achieves 92.9% coverage with table-driven tests.

The `OperationError` propagating exit codes from wrapped errors (`errors.go:250-259`) is a nice detail — a sync failure wrapping an API error correctly returns `ExitNetwork` instead of `ExitGeneric`.

**MH-6: Output tests require excessive save/restore boilerplate — SEVERITY: High**

`internal/output/output_test.go` — nearly every test function contains:
```go
oldWriter := Writer
oldFormat := CurrentFormat
oldLevel := Level
defer func() {
    Writer = oldWriter
    CurrentFormat = oldFormat
    Level = oldLevel
}()
```

This appears ~25 times in the file. It's a code smell that indicates the output system should support injection rather than relying on global state. Even a simple `withTestOutput(t, func(buf *bytes.Buffer) { ... })` helper would eliminate the repetition and prevent tests from accidentally leaking state.

At HashiCorp, we'd use a `*testing.T` cleanup pattern:
```go
func testOutput(t *testing.T) *bytes.Buffer {
    t.Helper()
    var buf bytes.Buffer
    old := Writer
    Writer = &buf
    t.Cleanup(func() { Writer = old })
    return &buf
}
```

**MH-7: No test for concurrent state access — SEVERITY: Medium**

`internal/state/state.go` implements file locking for concurrent access, but `state_test.go` never tests concurrent reads/writes. Given that weg uses `syscall.Flock` specifically for this purpose, there should be a test that:
1. Starts two goroutines
2. One writes state repeatedly
3. The other reads state repeatedly
4. Verifies no corruption or panics

The `-race` flag in CI (FACT-F9) helps, but explicit concurrency tests document the intended behavior.

**MH-8: Integration tests are in a separate `tests/` directory — good pattern — SEVERITY: Positive**

`tests/integration_test.go` is in its own package, imports internal packages, and tests cross-cutting workflows. `TestRemoteSiteSetupWorkflow` (lines 29-93) tests a complete 5-step workflow: create config → verify detection → save credentials → reload → check gitignore. `TestOutputFormatConsistency` (lines 96-186) tests all four output formats with the same data.

This is the right structure. The next step is adding CLI-level acceptance tests in this directory.

**MH-9: No test for the `configureOutput` function — SEVERITY: Medium**

`cmd/root.go:191-234` — `configureOutput()` has complex precedence logic: `--log-level` > `-q` > `-v count` > env var. This function is critical — it determines the entire output behavior of the CLI — but it's not tested directly. The `cmd/root_test.go` tests check flag defaults (lines 31-64) but don't exercise the precedence chain.

A test like "set both `-q` and `-vvv`, verify quiet wins" or "set `WEG_LOG_LEVEL=debug` and `-q`, verify `-q` wins" would catch regressions in this important behavior.

### Recommendations

1. **Create `internal/testutil` package** with:
   - `NewBench(t)` builder for creating test bench directories
   - `CaptureOutput(t)` for output testing without save/restore
   - `RunCommand(t, args)` for executing cobra commands and capturing results
   - `GoldenFile(t, name, got)` for golden file comparison

2. **Write behavioral tests for top 10 commands** (by expected usage frequency):
   - `weg site list` (empty, one site, multiple sites, JSON/table/quiet formats)
   - `weg site new` (success, already exists, invalid name)
   - `weg app list` (empty, with apps)
   - `weg api get` (local mode mock, format output)
   - `weg status` (each context type)
   - `weg config show` (both config file types)
   - `weg cache clear` (verify correct calls)
   - `weg build` (flag parsing, app selection)
   - `weg start`/`weg stop` (process management verification)
   - `weg remote status` (diff display)

3. **Add concurrent state test** to validate the `syscall.Flock` implementation under contention.

4. **Add `configureOutput` precedence tests** covering all combinations of verbosity flags, `--log-level`, and `WEG_LOG_LEVEL` env var.

5. **Adopt golden file testing** for CLI output — store expected outputs as testdata files, compare against actual output. This catches formatting regressions that unit tests miss.

6. **Add `--dry-run` integration tests** — commands supporting `--dry-run` (like `weg sync`) should have tests verifying that dry-run produces output but makes no changes.

---

## Consensus Findings

The three reviewers converge on these key observations:

### Strengths (Unanimous)

1. **Error type system is production-quality** (MH-5, GVR implicit approval). Eight typed errors with exit codes, wrapping, classification, and 92.9% test coverage. This is the best-tested package in the project and should be the template for others.

2. **Output system design is thoughtful** (SR-4, MH-4 related). Auto-detection of TTY/pipe, five formats, five verbosity levels, debug categories, and automatic secret redaction. The design is excellent; the implementation just needs the global state cleaned up.

3. **State management is robust** (MH-4). File locking, atomic writes, validation, diff computation. This is infrastructure-grade code.

4. **API command help text is exemplary** (SR-2). The `weg api` Long description should be the template for every command group.

### Weaknesses (Unanimous)

1. **CMD-level tests are structural, not behavioral** (MH-1, SR-3 related, GVR implicit). The entire cmd/ layer — which is where users interact with weg — has effectively 0% behavioral test coverage. This is the single biggest quality risk.

2. **Implicit context detection without visibility** (GVR-1, SR-6 related, MH implicit). Users have no easy way to see what weg detected and why. The five-context system is powerful but opaque.

3. **Documentation is uneven** (SR-3, SR-6, SR-7). The README and api help are great; site/app/other command help is minimal; CLI_CONVENTIONS.md exists but isn't discoverable.

### Critical Path

| Priority | Finding | Impact | Effort |
|----------|---------|--------|--------|
| P0 | MH-1: Behavioral cmd tests | Blocks confidence in releases | High |
| P0 | MH-3: Shared test helpers | Blocks efficient test writing | Medium |
| P1 | GVR-1: Context visibility | Blocks user trust in modes | Low |
| P1 | SR-3: Command help parity | Blocks documentation quality | Medium |
| P2 | GVR-4: skipAutoChdir redesign | Maintenance hazard | Low |
| P2 | MH-6: Output test helpers | Test maintenance cost | Low |
| P2 | GVR-3: api vs doc clarification | User confusion | Low |

---

## Proposed ADRs

### ADR-005: Adopt Behavioral Testing for CLI Commands

**Status:** Proposed
**Context:** cmd/ packages have <10% coverage, with existing tests verifying command registration rather than behavior. Users interact exclusively through the CLI surface, making this the highest-risk untested area.
**Decision:** Implement behavioral tests using cobra's `SetArgs`/`SetOut`/`SetErr` to test complete command execution against fixture directories. Create `internal/testutil` with bench/app fixture builders and output capture helpers. Target 60% behavioral coverage for cmd/ by v0.2.
**Consequences:** Test suite will be slower (filesystem operations) but much more meaningful. Golden files will catch formatting regressions.

### ADR-006: Make Context Detection Explicit and Visible

**Status:** Proposed
**Context:** The five-context detection system (`ContextFresh`, `ContextApp`, `ContextBench`, `ContextWegApp`, `ContextWegBench`) is implicit. Users cannot easily see which context weg detected, what config file it's using, or what bench path it resolved.
**Decision:** Enhance `weg status` (or add `weg context`) to display: detected context, config file path, bench path, default site, available sites, installed apps, and credential resolution path. Add warnings when ambiguous signals exist.
**Consequences:** Users can debug unexpected behavior. Error messages can reference `weg status` for diagnosis.

### ADR-007: Standardize Command Help Text Quality

**Status:** Proposed
**Context:** Help text quality varies from excellent (api, with modes, credential hierarchy, 8 examples) to minimal (site, with 4 lines restating subcommand names). Users rely on `--help` as primary documentation.
**Decision:** Every command group must include in its Long description: (1) conceptual overview, (2) relationship to other commands, (3) at least 3 examples. Destructive commands must show confirmation/force examples. Add CI lint to verify `Long` field is non-empty for all registered commands.
**Consequences:** More consistent onboarding experience. Help text becomes reliable documentation.

### ADR-008: Replace Global Output State with Injected Configuration

**Status:** Proposed
**Context:** The `output` package uses 7 package-level mutable globals, requiring every test to save/restore state manually (~25 instances in output_test.go alone). This prevents parallel test execution and makes the codebase fragile.
**Decision:** Introduce an `OutputConfig` struct that can be passed through cobra command context. The global variables remain as the default path for backward compatibility, but new code should use the injected config. Test helpers should create isolated configs.
**Consequences:** Tests become simpler and parallelizable. Gradual migration — no big-bang refactor needed.

---

## TDD Strategy Recommendations

### Phase 1: Test Infrastructure (Week 1-2)

**Create `internal/testutil` package with:**

```go
// BenchFixture creates a complete test bench directory
type BenchFixture struct {
    Path      string
    Sites     []string
    Apps      []string
    ConfigType string // "weg.toml" or "pyproject.toml"
}

func NewBench(t *testing.T) *BenchBuilder { ... }

// CommandResult captures command execution output
type CommandResult struct {
    Stdout   string
    Stderr   string
    ExitCode int
    Err      error
}

func RunCommand(t *testing.T, args ...string) *CommandResult { ... }

// OutputCapture provides isolated output testing
func CaptureOutput(t *testing.T) *bytes.Buffer { ... }

// GoldenFile comparison
func AssertGolden(t *testing.T, name string, got string) { ... }
```

### Phase 2: Critical Path Tests (Week 2-4)

**Priority 1 — Commands users run first:**
- `weg status` — all 5 contexts, correct output per format
- `weg site list` — empty bench, populated bench, all output formats
- `weg site new` — success path, duplicate name, validation errors
- `weg app list` — empty, populated, format variants

**Priority 2 — Commands that change state:**
- `weg site drop` — confirmation flow, `--force`, nonexistent site
- `weg app get` — URL parsing (tested in unit test, need behavioral test)
- `weg sync` — dry-run, actual sync, config hash update
- `weg cache clear` — subprocess invocation

**Priority 3 — API and remote commands:**
- `weg api get` — mock local executor, test output formatting
- `weg remote status` — mock remote client, test diff display
- `weg config show` — both config types

### Phase 3: Regression Prevention (Ongoing)

1. **Golden files** for all list/show command output (store in `testdata/`)
2. **Flag precedence tests** for `configureOutput` (all verbosity combinations)
3. **Concurrent state tests** (goroutine contention on state.json)
4. **Help text lint** in CI: verify all commands have Long description and Examples
5. **Error message tests**: verify each error type produces expected message format

### Testing Patterns to Adopt

| Pattern | Where | Why |
|---------|-------|-----|
| Table-driven tests | Already used in errors, config, completion | Expand to cmd/ |
| Test fixtures (builder) | New `testutil` package | Reduce test setup boilerplate |
| Golden files | cmd/ output tests | Catch formatting regressions |
| `t.Cleanup` for state | Replace manual defer chains in output tests | Reduce 25 instances of save/restore |
| Parallel tests | After removing global state dependency | Faster CI |
| Integration test package | Already exists in `tests/` | Add CLI-level tests here |

### Test Coverage Targets

| Package | Current | Target (v0.2) | Target (v1.0) |
|---------|---------|---------------|---------------|
| internal/errors | 92.9% | 95% | 95% |
| internal/completion | 84.4% | 85% | 90% |
| internal/config | 77.5% | 80% | 85% |
| internal/state | 76.3% | 85% | 90% |
| internal/output | ~60% | 75% | 85% |
| cmd/site | 4.5% | 40% | 60% |
| cmd/app | 7.4% | 40% | 60% |
| cmd/api | ~5% | 30% | 50% |
| internal/api | 15.0% | 40% | 60% |
| internal/remote | 37.8% | 50% | 65% |

### What NOT to Test

- Don't test cobra's flag parsing (it's well-tested upstream)
- Don't test `os.Chdir` behavior (test the detection logic instead)
- Don't write tests for simple getters/setters (like `GetProjectRoot`)
- Don't mock syscall.Flock in unit tests (test at integration level)
- Don't add tests for command registration (the existing structural tests are sufficient; invest effort in behavioral tests instead)
