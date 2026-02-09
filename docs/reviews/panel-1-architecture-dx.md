# Expert Panel 1: Architecture & Developer Experience Review

**Date:** 2026-02-09
**Reviewers:** Linus Torvalds (simulated), DHH (simulated), Armin Ronacher (simulated)
**Project:** weg — Go CLI for Frappe development

---

## Facts Used in This Review

| ID | Fact |
|----|------|
| F1 | weg is a Go 1.24 CLI for Frappe development (replacement for `bench` CLI) |
| F2 | 222 Go files, ~35k LOC code + ~10k LOC tests, 70+ commands across 20 groups |
| F3 | Uses cobra for CLI, 7 direct dependencies only |
| F4 | Three modes: app-centric (pyproject.toml), bench-centric (weg.toml), remote-site |
| F5 | Custom error types with exit codes (8 types), proper error wrapping |
| F6 | State management via .weg/state.json with file locking (syscall.Flock) and atomic writes |
| F7 | MCP server for AI integration (12 tools, subprocess + in-process handlers) |
| F8 | Output system supports json/table/plain/quiet with automatic secret redaction |
| F9 | CI/CD with gofmt, go vet, race detector tests, multi-platform release |
| F10 | 1 GitHub star, no releases yet, no issues, sole maintainer |
| F11 | Author's top repos: awesome-frappe (682 stars), contributions to frappe (9.6k stars) and erpnext (31.6k stars) |
| F12 | Pre-commit hook enforces gofmt |
| F13 | Test coverage varies: internal/errors 92.9%, internal/completion 84.4%, but cmd/site 4.5%, cmd/app 7.4% |
| F14 | Has PRODUCT_ROADMAP.md, USAGE.md, CLI_CONVENTIONS.md, DESIGN_SYSTEM_REFACTOR.md |

**New facts discovered during review:**

| ID | Fact |
|----|------|
| F15 | Global mutable state throughout `output` package (CurrentFormat, Level, DebugCategories, NoColor, ShowTimestamps, Writer, ErrWriter — 7 package-level vars at `internal/output/output.go:55-77`) |
| F16 | `cmd/root.go` uses `os.Chdir()` in PersistentPreRunE (line 91, 123) — process-wide side effect |
| F17 | `addAppToWegToml` in `cmd/app/get.go:136-147` is a stub that prints instructions instead of actually writing config |
| F18 | `cmd/version.go:138-144` implements custom `indexOf` function instead of using `strings.Index` |
| F19 | State `AppNames()` and `SiteNames()` claim to return "sorted" lists (comments at lines 278, 287) but don't sort — iterating a map |
| F20 | `cmd/helpers.go` duplicates bench-path resolution logic that also exists in `cmd/mcp/handlers.go:288-296, 349-356` |
| F21 | Container image builder tries podman first, then docker (`internal/container/image.go:316-317`) — surprising default order |
| F22 | `HasWegSection` in `internal/config/pyproject.go:105-122` does a full TOML parse just to check for section existence |
| F23 | MCP handler `handleWegExec` passes user input through `strings.Fields()` which could split quoted arguments incorrectly (`cmd/mcp/handlers.go:127`) |
| F24 | `internal/output/output.go:121-132` manually splits strings on commas to "avoid importing strings" — strings is already imported elsewhere in the package |

---

## Linus Torvalds' Review

### Summary

The bones are good. Seven direct dependencies for a CLI with 70+ commands — that's discipline most Go projects lack. The file locking with `syscall.Flock` and atomic writes via temp-file-plus-rename shows someone who actually understands how filesystems work, not just someone who read a blog post. But there's a pervasive problem with global mutable state that will bite you the moment you want to test anything properly or run concurrent operations. The `os.Chdir()` in the command prerun is the kind of thing that makes me want to throw things — it's a process-wide side effect hidden in initialization code. And the custom `indexOf` function when `strings.Index` exists? That's not minimalism, that's cargo-cult programming.

### Findings

#### LT-1: Process-wide `os.Chdir()` in command initialization [Severity: High]

**File:** `cmd/root.go:91, 123`

The `PersistentPreRunE` calls `os.Chdir()` twice — once for the explicit `--chdir` flag and once for auto-detected project root. This is a process-wide side effect. Every goroutine, every deferred cleanup, everything that assumed the working directory is stable now has the rug pulled out from under it. Git figured this out decades ago — you pass the working directory to operations, you don't `chdir()` the entire process.

```go
// root.go:91 - This changes the process CWD
if chdir != "" {
    if err := os.Chdir(chdir); err != nil {
        return fmt.Errorf("failed to change directory to %s: %w", chdir, err)
    }
}
```

The `skipAutoChdir` map on line 97-109 is a band-aid on the real problem. When you need a blocklist of commands that shouldn't chdir, you've already lost the design argument.

**Evidence:** The fact that `originalDir` is stored (line 55-56) to remember where you came from proves the author knows this is fragile.

#### LT-2: Global mutable state in `output` package [Severity: High]

**File:** `internal/output/output.go:55-77`

Seven mutable package-level variables: `CurrentFormat`, `Level`, `DebugCategories`, `NoColor`, `ShowTimestamps`, `Writer`, `ErrWriter`. This is shared mutable state with zero synchronization. The test for `errors_test.go:287-288` already shows the problem — tests must save and restore `output.ErrWriter` manually.

```go
var (
    CurrentFormat Format = FormatAuto
    Level Verbosity = VerbosityNormal
    DebugCategories map[DebugCategory]bool
    NoColor bool
    ShowTimestamps bool = true
    Writer io.Writer = os.Stdout
    ErrWriter io.Writer = os.Stderr
)
```

The correct pattern is a config struct passed through context or threaded as a parameter. The `slog` package in the standard library got this right.

#### LT-3: Unsorted "sorted" functions [Severity: Medium]

**File:** `internal/state/state.go:278-294`

Both `AppNames()` and `SiteNames()` have comments claiming they "return a sorted list" but the implementation just iterates a map — which in Go gives you random order:

```go
// AppNames returns a sorted list of app names   <-- LIE
func (s *State) AppNames() []string {
    names := make([]string, 0, len(s.Apps))
    for name := range s.Apps {
        names = append(names, name)  // Random map iteration order
    }
    return names
}
```

This is worse than not sorting — it's a function that lies about its contract. Anyone depending on this for deterministic output will get intermittent test failures. Add `sort.Strings(names)` or fix the comment.

#### LT-4: Reinventing `strings.Index` [Severity: Low]

**File:** `cmd/version.go:138-144`

```go
func indexOf(s, substr string) int {
    for i := 0; i <= len(s)-len(substr); i++ {
        if s[i:i+len(substr)] == substr {
            return i
        }
    }
    return -1
}
```

This is `strings.Index()` but worse — it creates substring allocations on every iteration. The standard library function uses Rabin-Karp. Use it.

#### LT-5: File locking — good pattern, incomplete implementation [Severity: Medium]

**File:** `internal/state/state.go:74-98`

The locking pattern is actually reasonable — shared lock for reads, exclusive lock for writes, separate lock file. But the fallback on lines 87-88 and 92-93 silently drops the lock and proceeds without it:

```go
if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_SH); err != nil {
    // Fall back to unlocked read if locking fails
    return loadStateUnlocked(statePath)
}
```

If locking fails, something is genuinely wrong (filesystem doesn't support it, or resource exhaustion). Silently proceeding without locking is how you get corrupted state. At minimum, log a warning. Better: return the error and let the caller decide.

#### LT-6: Dockerfile as a Go string literal [Severity: Medium]

**File:** `internal/container/image.go:41-214`

A 173-line Dockerfile embedded as a string literal in Go code. No syntax highlighting, no linting, no way to test it independently. When someone needs to debug a Docker build issue, they'll have to mentally parse Go string escaping on top of Dockerfile syntax.

This should be an embedded file (Go 1.16 `//go:embed`) or a template. The current approach is the worst of both worlds — you get neither the benefits of Go type checking nor the benefits of a standalone Dockerfile.

#### LT-7: Podman-first container detection [Severity: Low]

**File:** `internal/container/image.go:316-317`

```go
builder := "docker"
if _, err := exec.LookPath("podman"); err == nil {
    builder = "podman"
}
```

This silently prefers podman over docker if both are installed. That's a surprising default that will confuse 90% of users who expect `docker` when they type a docker command. The detection should be: check for a `CONTAINER_RUNTIME` env var, then check what the user last used, then default to docker.

### Recommendations

1. **Replace `os.Chdir` with explicit path threading.** Store the resolved project root and pass it to every function that needs it. Git does this with `GIT_WORK_TREE` and `GIT_DIR` — never `chdir`.
2. **Create an `OutputConfig` struct** passed via context or function parameter. Kill the package-level mutable globals.
3. **Fix or remove the lying sort comments.** Either call `sort.Strings()` or change the doc comment to say "returns names in arbitrary order."
4. **Use `//go:embed` for the Dockerfile template.** Gain syntax highlighting, independent testing, and IDE support.
5. **Make lock failure an error, not a fallback.** Silent degradation of safety guarantees is worse than a loud failure.

---

## DHH's Review

### Summary

There's a lot to like about the fundamental concept. Replacing an imperative Python CLI with a declarative TOML-based Go tool? That's the right instinct — convention over configuration, fast feedback loops, fewer moving parts. The three-mode architecture (app-centric, bench-centric, remote) is genuinely clever and maps well to real developer workflows. But the current implementation suffers from premature generalization in some places and stubbed-out features in others. The `addAppToWegToml` function that just prints "Note: Add the following..." is the kind of thing that makes developers lose trust in a tool immediately. Either the tool manages your config or it doesn't — pick one.

### Findings

#### DHH-1: Config management tells the user to do it themselves [Severity: Critical]

**File:** `cmd/app/get.go:136-147`

```go
func addAppToWegToml(path, name, url, branch string) error {
    // This is a simplified version - in practice would use proper TOML manipulation
    // For now, we just note that the config should be updated
    fmt.Printf("Note: Add the following to weg.toml:\n\n")
    fmt.Printf("[apps.%s]\n", name)
    fmt.Printf("url = \"%s\"\n", url)
    if branch != "" {
        fmt.Printf("branch = \"%s\"\n", branch)
    }
    fmt.Println()
    return nil
}
```

This is a fundamental betrayal of the declarative promise. The whole point of `weg` is that `weg.toml` is the source of truth. When `weg app get` installs an app but tells the user to manually edit weg.toml, you've broken the single most important contract your tool makes. Rails wouldn't add a migration but then tell you to manually write the SQL — the tooling does it for you.

This function also always returns `nil`, making its caller's error handling (`cmd/app/get.go:101`) dead code.

#### DHH-2: Two config formats with divergent schemas [Severity: High]

**Files:** `internal/config/wegtoml.go`, `internal/config/pyproject.go`

`BenchConfig` (weg.toml) and `AppConfig` (pyproject.toml) have completely different schemas with no shared abstraction. `BenchConfig` uses `Apps map[string]AppSettings` while `AppConfig` uses `Dependencies.Apps []AppDependency`. The field names differ (`url` vs `url`, `branch` vs `branch` — same names but different parent structures). There's no common interface or adapter.

This means every command that reads configuration has to handle both formats independently. That's not convention over configuration — that's two conventions fighting each other. Consider a unified `ProjectConfig` interface that both formats implement, so commands don't need to know which config file they're reading from.

#### DHH-3: 70+ commands but critical ones are stubs [Severity: High]

**Files:** `cmd/site/site.go:20-26`, `cmd/app/get.go:136-147`

The `site.go` parent command registers 5 subcommands (list, new, drop, use, install), but the coverage is 4.5%. Meanwhile, the README advertises `weg site browse`, `weg site backup`, `weg site restore`, `weg site password` which appear to not exist yet. The command tree looks impressive on paper but the actual implementation depth varies wildly.

This is the "fifty half-finished features" problem. I'd rather have 20 commands that work perfectly than 70 that work "mostly." Focus the command surface area on what's complete and mark the rest explicitly as `[planned]` in help output.

#### DHH-4: Excellent CLI ergonomics on flags and output [Severity: N/A — Positive]

**Files:** `cmd/root.go:148-155`, `internal/output/output.go`, `internal/output/symbols.go`

Credit where due: the `--output auto|json|table|plain|quiet` flag with automatic TTY detection (`output.go:167-175`) is exactly right. The auto-detect of JSON when piped vs table when interactive means shell scripts and humans both get what they want without explicit flags. The `-v`, `-vv`, `-vvv` verbosity escalation follows established convention (like `curl -v`). The Unicode status symbols (checkmarks, arrows, warnings at `symbols.go:8-15`) give visual structure without being noisy.

This is the level of polish every command should have.

#### DHH-5: Environment detection is smart but risky [Severity: Medium]

**File:** `internal/config/detect.go:52-116`

The `DetectContext` function walks through 6 possible contexts with a clear priority order. The `SuggestAction` method (line 253-268) that tells users what to do next is great DX. But the detection logic depends on the presence of files like `hooks.py` which is a Frappe convention that could change in future versions. There's no escape hatch — no `WEG_CONTEXT=bench` env var override.

The `hasHooksPy` function (line 189-220) scanning subdirectories for hooks.py is especially fragile. It skips `node_modules`, `docs`, `tests` by name (line 209) but any new directory convention (like `vendor/`) would break it.

#### DHH-6: The "skip auto chdir" blocklist [Severity: Medium]

**File:** `cmd/root.go:97-109`

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

This is an allowlist that's actually a denylist — it lists commands that should NOT auto-chdir. Every new command requires asking "should this be in the skip list?" That's the opposite of convention over configuration. The default should be "commands work in the current directory" with auto-chdir being opt-in for the few commands that need it, not the other way around.

#### DHH-7: Naming friction — "weg" meaning varies by audience [Severity: Low]

**File:** `README.md:7`

"Weg means 'way' in German and 'speed' in Marathi/Sanskrit" — but to English speakers, "weg" doesn't mean anything. The README has to explain it. Compare with tools like `brew`, `cargo`, `mix` — all English words with intuitive metaphors. This isn't fatal but it's friction for adoption. More importantly, the dual-meaning suggests the project is trying to be clever rather than clear.

### Recommendations

1. **Finish `addAppToWegToml` before shipping.** Use `BurntSushi/toml` (already a dependency) to read-modify-write the TOML file. This is table stakes.
2. **Create a `ProjectConfig` interface** that unifies `BenchConfig` and `AppConfig`. Commands should operate on the interface, not concrete types.
3. **Prune the command tree.** Mark unfinished commands as hidden or `[planned]`. Don't let the README promise features the code can't deliver.
4. **Flip the chdir default.** Make auto-chdir opt-in per command via a cobra annotation, not opt-out via a blocklist.
5. **Add a `WEG_CONTEXT` environment variable** for explicit context override, useful in CI/CD and edge cases.

---

## Armin Ronacher's Review

### Summary

The project shows thoughtful design in several areas — the error type hierarchy with exit codes, the output redaction system, and the MCP server architecture are all above-average for a pre-release CLI. But there are concerning gaps in observability, error propagation, and the MCP security surface. The redactor has good coverage of field names but the regex-based value detection is fragile. The MCP `weg_exec` tool passes unsanitized user input through `strings.Fields()`, which is an argument injection vector. The debug/trace system in `output/debug.go` is well-designed but underused — I see almost no instrumentation in the actual command implementations. Having the infrastructure for observability without using it is worse than not having it, because it creates a false sense of coverage.

### Findings

#### AR-1: MCP `weg_exec` allows argument injection [Severity: Critical]

**File:** `cmd/mcp/handlers.go:118-134`

```go
func handleWegExec(_ context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
    command, err := request.RequireString("command")
    // ...
    args := []string{"exec"}
    args = append(args, siteArgs(site)...)
    args = append(args, strings.Fields(command)...)  // LINE 127

    out, err := runWegCommand(args...)
```

The `strings.Fields(command)` splits on whitespace, which means:
- Quoted arguments are broken: `"frappe --site 'my site'"` becomes 4 separate args
- The user of the MCP tool (an AI assistant) could pass arbitrary weg subcommand arguments

The same pattern exists in `handleWegApiCall` (line 108): `strings.Fields(extraArgs)...`.

This should use proper shell tokenization (`shlex`-equivalent) or, better, accept structured arguments instead of a freeform command string. At minimum, document the limitation clearly in the tool description.

#### AR-2: Error context lost in MCP handlers [Severity: High]

**File:** `cmd/mcp/handlers.go` (throughout)

Every MCP handler follows this pattern:

```go
out, err := runWegCommand(args...)
if err != nil {
    return mcplib.NewToolResultError(err.Error()), nil
}
```

The `err.Error()` call stringifies the error, losing the structured error types from `internal/errors`. The AI consumer of this MCP server gets a flat string like "sync failed: failed to connect: timeout" with no way to distinguish a retryable error from a permanent one.

Consider returning structured error information in the MCP response — at minimum, an error code alongside the message:

```json
{"error": "sync failed", "code": "network", "retryable": true}
```

The `IsRetryable()` and `IsUserError()` functions in `internal/errors/errors.go:302-334` already exist for this purpose but are never used in MCP handlers.

#### AR-3: Redactor has bypass vectors [Severity: Medium]

**File:** `internal/output/redact.go`

The `IsSecretField` check is case-insensitive (good), but only matches exact field names. A field named `db_password_hash` or `api_token_v2` would not be caught because the map lookups use exact matches against normalized names, and neither "db_password_hash" nor "api_token_v2" is in the map (line 26-51).

The regex patterns (line 54-61) only catch Bearer tokens, Basic auth, and `key:secret` format. JWT tokens (`eyJ...`), AWS credentials (`AKIA...`), and GitHub tokens (`ghp_...`, `gho_...`) all pass through unredacted.

The `RedactJSON` function (line 155-160) applies string-level regex to raw JSON, which means it can break JSON structure if a match spans a key-value boundary.

For a tool that bridges Python apps (which handle sensitive data) with AI assistants (which have logging), the redaction surface should be broader.

#### AR-4: Debug infrastructure exists but isn't wired up [Severity: Medium]

**Files:** `internal/output/debug.go`, all `cmd/` files

The debug system is well-designed: categorized output, timestamps, HTTP trace logging with automatic header redaction, `WithTiming` for operation profiling. But from reading the actual command implementations (`cmd/app/get.go`, `cmd/site/site.go`, `cmd/helpers.go`), I see zero calls to `output.Debug()`, `output.Debugf()`, or `output.WithTiming()`.

The `TraceHTTPRequest`/`TraceHTTPResponse` helpers (debug.go:130-175) suggest the intent to instrument HTTP calls, but without seeing them called from the actual HTTP client code, they're dead infrastructure.

This is a classic observability anti-pattern: building the logging framework before the application, then never instrumenting the application. The `-vvv` flag will produce no useful output if nothing calls the trace functions.

#### AR-5: `handleWegSiteList` loads state from wrong path [Severity: Medium]

**File:** `cmd/mcp/handlers.go:277-335`

```go
absPath, err := filepath.Abs(".")
// ...
var benchPath string
switch result.Context {
case config.ContextWegApp:
    benchPath = filepath.Join(absPath, ".weg")  // benchPath set correctly
case config.ContextWegBench:
    benchPath = absPath
}

st, err := state.Load(absPath)  // <-- Bug: loads from absPath, not benchPath
```

The `benchPath` is computed correctly (line 291-295) but then `state.Load()` on line 298 uses `absPath` instead of `benchPath`. For bench-centric projects they're the same, but for app-centric projects (`ContextWegApp`), the state file is in `.weg/` not in the project root. The same bug exists in `handleWegAppList` at line 358.

The fallback to scanning the filesystem (lines 319-331) masks this bug in practice — if state loading returns empty, it scans the directory. But that means state information (like which site is default) is silently lost.

#### AR-6: No context cancellation or timeout in MCP handlers [Severity: Medium]

**File:** `cmd/mcp/handlers.go:18-33`

```go
func runWegCommand(args ...string) (string, error) {
    exe, err := os.Executable()
    // ...
    cmd := exec.Command(exe, args...)
    out, err := cmd.CombinedOutput()
```

The `context.Context` parameter is accepted by every handler (e.g., line 46: `func handleWegPy(_ context.Context, ...`) but never used — it's discarded with `_`. The `exec.Command` should be `exec.CommandContext` so that if the MCP client disconnects, the subprocess is killed.

Without this, a slow `weg test` or `weg build` called via MCP will run to completion even if the AI assistant has moved on, wasting resources.

#### AR-7: Duplicate bench-path resolution logic [Severity: Low]

**Files:** `cmd/helpers.go:40-47`, `cmd/mcp/handlers.go:288-296`

The switch statement to resolve bench path from context:
```go
switch result.Context {
case config.ContextWegBench:
    benchPath = absPath
case config.ContextWegApp:
    benchPath = filepath.Join(absPath, ".weg")
}
```

...appears in `ResolveBenchPathFrom`, `handleWegSiteList`, `handleWegAppList`, `runGet`, and `handleWegStatus` — five locations with slightly different error handling for each. This should be a single `ResolveBenchPath(result)` utility that all callers use.

### Recommendations

1. **Fix MCP argument injection.** Replace `strings.Fields()` with a proper tokenizer, or restructure `weg_exec` to accept an args array instead of a freeform string. Consider whether `weg_exec` should exist at all — it's an escape hatch that undermines the structured tool approach.
2. **Propagate structured errors through MCP.** Return error type, code, and retryability in MCP error responses so AI consumers can make intelligent retry decisions.
3. **Expand redaction patterns.** Add AWS key prefixes (`AKIA`), GitHub token prefixes (`ghp_`, `gho_`, `github_pat_`), JWT detection (`eyJ`), and substring matching for field names (catch `db_password` not just `password`).
4. **Wire up the debug system.** Add `output.Debugf(output.DebugExec, ...)` calls around every subprocess execution, `output.WithTiming` around config parsing and state loading. The infrastructure is good — use it.
5. **Use `exec.CommandContext`** in `runWegCommand` and pass through the MCP handler's context parameter.
6. **Fix the state loading path bug** in `handleWegSiteList` and `handleWegAppList` — use `benchPath` instead of `absPath` for `state.Load()`.

---

## Consensus Findings

The following findings were identified by 2 or more reviewers:

### 1. Global mutable state is the root problem (LT-2, DHH-6, AR-4)
**Agreed by:** Linus, DHH, Armin

The output package globals, the process-wide `os.Chdir()`, and the skip-list pattern all stem from the same design choice: global state instead of explicit dependency threading. This makes testing fragile (errors_test.go manually saves/restores globals), prevents concurrent use, and creates implicit coupling between unrelated packages.

### 2. Stub implementations shipped as features (DHH-1, DHH-3)
**Agreed by:** DHH, Armin

The `addAppToWegToml` stub and the low test coverage of command packages indicate features that were scaffolded but not completed. The README advertises capabilities (site browse, backup, restore) that may not exist.

### 3. MCP security surface needs hardening (AR-1, AR-2, LT-1)
**Agreed by:** Linus, Armin

The MCP server is an attack surface multiplier — it exposes CLI functionality to external AI agents. The argument injection in `weg_exec`, loss of structured error information, and lack of context cancellation all represent real risks that compound when an AI agent is the caller.

### 4. Duplicated bench-path resolution (AR-7, F20)
**Agreed by:** DHH, Armin

The same switch-on-context pattern appears 5+ times across the codebase. This violates DRY and leads to subtle bugs (AR-5: wrong path in MCP handlers).

### 5. Strong fundamentals — dependencies, error types, output system (LT-5 positive, DHH-4, AR-3 partial)
**Agreed by:** All three

The 7-dependency constraint, the custom error hierarchy with exit codes, and the multi-format output system are all well above average for a pre-release CLI. The project has a solid foundation that the above issues sit on top of.

---

## Proposed ADRs

Based on this review, the following Architecture Decision Records should be created:

1. **ADR: Replace `os.Chdir()` with explicit path threading**
   - Addresses: LT-1, DHH-6
   - Scope: `cmd/root.go`, all command implementations

2. **ADR: Introduce `OutputConfig` struct to replace output package globals**
   - Addresses: LT-2
   - Scope: `internal/output/`, all consumers

3. **ADR: Unified `ProjectConfig` interface for weg.toml and pyproject.toml**
   - Addresses: DHH-2, AR-7
   - Scope: `internal/config/`, all command implementations

4. **ADR: MCP security hardening — input validation, structured errors, context propagation**
   - Addresses: AR-1, AR-2, AR-6
   - Scope: `cmd/mcp/`

5. **ADR: Complete config write-back for `weg app get` and similar commands**
   - Addresses: DHH-1
   - Scope: `cmd/app/`, `internal/config/`

6. **ADR: Embed Dockerfile template using `//go:embed`**
   - Addresses: LT-6
   - Scope: `internal/container/`

7. **ADR: Expand secret redaction to cover modern token formats**
   - Addresses: AR-3
   - Scope: `internal/output/redact.go`

8. **ADR: Consolidate bench-path resolution into single utility**
   - Addresses: AR-7, F20
   - Scope: `cmd/helpers.go`, all consumers
