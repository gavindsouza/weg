# Weg CLI Design System Refactor Plan

**Created:** 2026-01-21
**Status:** Complete
**Scope:** Design system consistency, then logical inconsistencies

---

## Executive Summary

This document outlines a phased refactor to establish consistent patterns across the Weg CLI codebase. The refactor prioritizes foundational changes first to avoid rework.

### Current State Metrics

| Metric | Value |
|--------|-------|
| Total Go files | 207 |
| Files in `cmd/` | 155 |
| Total LoC | 39,633 |
| `fmt.Errorf` calls | 874 |
| Error checks (`if err != nil`) | 743 |
| Files needing updates | ~30 (19.4% of cmd/) |

### Key Problems

1. **No centralized output formatting** - 8 files duplicate tabwriter setup
2. **Inconsistent confirmations** - 19 files with 2 different patterns
3. **Single exit code** - All errors exit with code 1
4. **Mixed error prefixes** - "Error:", "Warning:", "failed to" patterns
5. **Flag conflicts** - `-f` means both `--force` and `--filters`

---

## Phase 1: Output Package (`internal/output`)

**Goal:** Centralize all user-facing output with a unified formatter supporting multiple output formats (JSON, table, plain text).

### 1.1 Core Concept: Unified Formatter

Commands should produce **structured data**, and the output package decides **how to render** it based on user preference (`--output` flag).

```
┌─────────────┐     ┌──────────────┐     ┌─────────────────┐
│   Command   │────▶│  Structured  │────▶│    Formatter    │
│  (produces) │     │    Data      │     │ (renders based  │
│             │     │              │     │  on --output)   │
└─────────────┘     └──────────────┘     └─────────────────┘
                                                  │
                          ┌───────────────────────┼───────────────────────┐
                          ▼                       ▼                       ▼
                    ┌──────────┐           ┌──────────┐           ┌──────────┐
                    │   JSON   │           │  Table   │           │  Plain   │
                    │ (script) │           │ (human)  │           │ (simple) │
                    └──────────┘           └──────────┘           └──────────┘
```

**Benefits:**
- Commands are scriptable (`weg site list --output json | jq ...`)
- Consistent formatting across all commands
- Easy to add new formats (YAML, CSV, etc.)
- Testable output (compare structured data, not strings)

### 1.2 New Package Structure

```
internal/output/
├── output.go       # Core types, config, Format enum
├── formatter.go    # Formatter interface and registry
├── table.go        # Table formatter (tabwriter)
├── json.go         # JSON formatter
├── plain.go        # Plain text formatter
├── symbols.go      # Unicode symbols and status indicators
├── result.go       # Result type for command output
└── output_test.go  # Tests
```

### 1.3 API Design

```go
package output

import (
    "io"
    "os"
    "text/tabwriter"
)

// ============================================================
// FORMAT ENUM (output.go)
// ============================================================

type Format string

const (
    FormatAuto  Format = "auto"   // Detect: JSON if piped, table if TTY
    FormatJSON  Format = "json"   // JSON output (for scripting)
    FormatTable Format = "table"  // Tabular output (for humans)
    FormatPlain Format = "plain"  // Simple text output
    FormatQuiet Format = "quiet"  // Minimal output (IDs only)
)

// ParseFormat converts string to Format, returns error if invalid
func ParseFormat(s string) (Format, error)

// ============================================================
// CONFIGURATION (output.go)
// ============================================================

var (
    // CurrentFormat is set by --output flag (default: auto)
    CurrentFormat Format = FormatAuto

    // Verbose enables debug output
    Verbose bool

    // Quiet suppresses non-essential output
    Quiet bool

    // NoColor disables colored output
    NoColor bool

    // Writer is the output destination (default: os.Stdout)
    Writer io.Writer = os.Stdout

    // ErrWriter is the error destination (default: os.Stderr)
    ErrWriter io.Writer = os.Stderr
)

// IsTTY returns true if Writer is a terminal
func IsTTY() bool

// EffectiveFormat returns the actual format to use
// (resolves "auto" to json or table based on TTY)
func EffectiveFormat() Format

// ============================================================
// RESULT TYPE (result.go)
// ============================================================

// Result represents command output that can be formatted
type Result struct {
    // Headers for table output (optional)
    Headers []string

    // Rows of data (each row is a map or struct)
    Rows []any

    // Single item output (for non-list commands)
    Item any

    // Message for plain text output
    Message string

    // Success indicator
    Success bool
}

// NewResult creates a new result for a single item
func NewResult(item any) *Result

// NewListResult creates a result for a list of items
func NewListResult(headers []string, rows []any) *Result

// NewMessage creates a result with just a message
func NewMessage(format string, args ...any) *Result

// Print outputs the result in the current format
func (r *Result) Print() error

// PrintTo outputs the result to a specific writer
func (r *Result) PrintTo(w io.Writer) error

// ============================================================
// CONVENIENCE FUNCTIONS (output.go)
// ============================================================

// List prints a list of items (auto-detects headers from struct tags)
// Usage: output.List(sites)  // sites is []Site with json tags
func List(items any) error

// Item prints a single item
// Usage: output.Item(site)
func Item(item any) error

// Message prints a simple message (respects Quiet flag)
func Message(format string, args ...any)

// ============================================================
// SYMBOLS (symbols.go)
// ============================================================

const (
    SymbolSuccess = "✓"
    SymbolError   = "✗"
    SymbolWarning = "⚠"
    SymbolInfo    = "→"
    SymbolPending = "○"
    SymbolActive  = "●"
)

// Status prints a status line with symbol (plain format only)
// In JSON mode, this is a no-op (status goes in result)
func Status(symbol, message string)
func Statusf(symbol, format string, args ...any)

// Success prints: ✓ message
func Success(message string)
func Successf(format string, args ...any)

// Error prints to stderr: ✗ message
func Error(message string)
func Errorf(format string, args ...any)

// Warning prints to stderr: ⚠ message
func Warning(message string)
func Warningf(format string, args ...any)

// Step prints a numbered step: [1/3] message
func Step(current, total int, message string)

// ============================================================
// TABLE (table.go) - Low-level API
// ============================================================

// Table wraps tabwriter with consistent defaults
type Table struct {
    w       *tabwriter.Writer
    headers []string
}

// NewTable creates a table writing to stdout
func NewTable(headers ...string) *Table

// NewTableWriter creates a table writing to custom writer
func NewTableWriter(w io.Writer, headers ...string) *Table

// Row adds a row to the table
func (t *Table) Row(values ...any)

// Flush writes the table
func (t *Table) Flush()

// ============================================================
// JSON (json.go) - Low-level API
// ============================================================

// JSON prints value as indented JSON (2 spaces)
func JSON(v any) error

// JSONTo prints value as indented JSON to writer
func JSONTo(w io.Writer, v any) error

// ============================================================
// DEBUG (output.go)
// ============================================================

// Debug prints only if Verbose is true
func Debug(message string)
func Debugf(format string, args ...any)
```

### 1.4 Command Integration Pattern

**Before (current):**
```go
func listSites(cmd *cobra.Command, args []string) error {
    sites, err := getSites()
    if err != nil {
        return err
    }

    w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
    fmt.Fprintln(w, "NAME\tSTATUS\tAPPS")
    for _, s := range sites {
        fmt.Fprintf(w, "%s\t%s\t%s\n", s.Name, s.Status, s.Apps)
    }
    w.Flush()
    return nil
}
```

**After (with unified formatter):**
```go
// Site struct with json tags for automatic header detection
type Site struct {
    Name   string `json:"name"`
    Status string `json:"status"`
    Apps   string `json:"apps"`
}

func listSites(cmd *cobra.Command, args []string) error {
    sites, err := getSites()
    if err != nil {
        return err
    }

    // Single line - formatter handles JSON vs Table vs Plain
    return output.List(sites)
}
```

**Output examples:**

```bash
# Default (TTY) - table format
$ weg site list
NAME              STATUS    APPS
mysite.localhost  Running   frappe, erpnext
test.localhost    Stopped   frappe

# Scripting - JSON format
$ weg site list --output json
[
  {"name": "mysite.localhost", "status": "Running", "apps": "frappe, erpnext"},
  {"name": "test.localhost", "status": "Stopped", "apps": "frappe"}
]

# Piped (auto-detected as JSON)
$ weg site list | jq '.[0].name'
"mysite.localhost"

# Plain format
$ weg site list --output plain
mysite.localhost (Running): frappe, erpnext
test.localhost (Stopped): frappe

# Quiet format (IDs only, for scripting)
$ weg site list --output quiet
mysite.localhost
test.localhost
```

### 1.5 Global Flag Addition

Add to `cmd/root.go`:

```go
var (
    outputFormat    string
    verboseCount    int      // Track -v flags
    logLevel        string   // --log-level flag
    debugCategories string   // --debug-categories=net,config
)

func init() {
    // Output format
    rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "auto",
        "Output format: auto, json, table, plain, quiet")

    // Verbosity - two equivalent approaches
    rootCmd.PersistentFlags().CountVarP(&verboseCount, "verbose", "v",
        "Increase verbosity (-v, -vv, -vvv)")
    rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "",
        "Set log level: quiet, normal, verbose, debug, trace")
    rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false,
        "Suppress non-essential output")

    // Debug category filter (works at debug/trace levels)
    rootCmd.PersistentFlags().StringVar(&debugCategories, "debug-categories", "",
        "Filter debug output: all,config,state,net,git,fs,exec")

    // Wire to output package in PersistentPreRun
    rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
        // Parse output format
        format, err := output.ParseFormat(outputFormat)
        if err != nil {
            return err
        }
        output.CurrentFormat = format

        // Determine verbosity level
        // Precedence: --log-level > -q > -v count > env var
        if logLevel != "" {
            level, err := output.ParseVerbosity(logLevel)
            if err != nil {
                return err
            }
            output.Level = level
        } else if quiet {
            output.Level = output.VerbosityQuiet
        } else if verboseCount >= 3 {
            output.Level = output.VerbosityTrace
        } else if verboseCount >= 2 {
            output.Level = output.VerbosityDebug
        } else if verboseCount >= 1 {
            output.Level = output.VerbosityVerbose
        } else {
            output.Level = output.VerbosityNormal
        }

        // Parse debug categories
        if debugCategories != "" {
            output.ParseDebugCategories(debugCategories)
        }

        // Check environment variables (lowest precedence, won't override flags)
        output.LoadFromEnv()

        return nil
    }
}
```

**Environment variable loading:**

```go
// LoadFromEnv reads verbosity settings from environment
func LoadFromEnv() {
    if level := os.Getenv("WEG_LOG_LEVEL"); level != "" {
        switch level {
        case "quiet":
            Level = VerbosityQuiet
        case "verbose":
            Level = VerbosityVerbose
        case "debug":
            Level = VerbosityDebug
        case "trace":
            Level = VerbosityTrace
        }
    }

    if cats := os.Getenv("WEG_DEBUG"); cats != "" {
        ParseDebugCategories(cats)
    }

    if os.Getenv("WEG_DEBUG_FORMAT") == "json" {
        DebugFormat = "json"
    }

    if os.Getenv("WEG_NO_COLOR") != "" || os.Getenv("NO_COLOR") != "" {
        NoColor = true
    }
}
```

### 1.6 Migration Path

**Files to update (8 tabwriter users):**
- `cmd/app/list.go`
- `cmd/cloud/benches.go`
- `cmd/cloud/marketplace.go`
- `cmd/cloud/sites.go`
- `cmd/cloud/status.go`
- `cmd/config/list_apps.go`
- `cmd/site/backup.go`
- `cmd/site/list.go`

**Migration pattern:**

| Before | After |
|--------|-------|
| `tabwriter.NewWriter(...)` | `output.List(items)` |
| `fmt.Println("✓ ...")` | `output.Success(...)` |
| `json.MarshalIndent(...)` | `output.Item(...)` with `--output json` |

**Files to update (35+ symbol users):**
- All files using `fmt.Println("✓ ...")`
- Grep pattern: `fmt\.Print.*[✓✗⚠]`

**Before:**
```go
fmt.Println("✓ Connected")
fmt.Printf("✓ Cloned to %s/\n", dir)
```

**After:**
```go
output.Success("Connected")
output.Successf("Cloned to %s/", dir)
```

### 1.7 Format Behavior Matrix

| Function | JSON | Table | Plain | Quiet |
|----------|------|-------|-------|-------|
| `List(items)` | Array | Tabwriter | One per line | IDs only |
| `Item(item)` | Object | Key-value | Formatted | ID only |
| `Success(msg)` | `{"status":"ok","message":...}` | `✓ msg` | `msg` | (none) |
| `Error(msg)` | `{"error":...}` | `✗ msg` | `Error: msg` | `Error: msg` |
| `Warning(msg)` | `{"warning":...}` | `⚠ msg` | `Warning: msg` | (none) |
| `Step(n,t,msg)` | (none) | `[n/t] msg` | `[n/t] msg` | (none) |
| `Debug(msg)` | (none) | `msg` | `msg` | (none) |

### 1.8 Verbosity & Debug System

**Goal:** Provide layered verbosity with structured debug output for troubleshooting.

#### Verbosity Levels

```go
type Verbosity int

const (
    VerbosityQuiet   Verbosity = -1  // Errors only
    VerbosityNormal  Verbosity = 0   // Default output
    VerbosityVerbose Verbosity = 1   // + Context, what's happening
    VerbosityDebug   Verbosity = 2   // + Internal state, timing
    VerbosityTrace   Verbosity = 3   // + Network, file ops, everything
)
```

#### Flag Mapping

Two ways to set verbosity - pick whichever fits your workflow:

| Quick (interactive) | Explicit (scripts) | Level | Output |
|---------------------|-------------------|-------|--------|
| `-q` | `--log-level=quiet` | Quiet | Errors and final result only |
| (default) | `--log-level=normal` | Normal | Standard operation output |
| `-v` | `--log-level=verbose` | Verbose | + "Loading config...", "Connecting to..." |
| `-vv` | `--log-level=debug` | Debug | + Timing, decisions, internal state |
| `-vvv` | `--log-level=trace` | Trace | + HTTP requests/responses, file reads |

**Precedence:** `--log-level` wins over `-v` count if both specified.

**Why two ways?**
- `-vv` is fast to type interactively
- `--log-level=debug` is self-documenting in scripts and CI configs

#### Debug Categories

For granular control, support category filtering:

```go
type DebugCategory string

const (
    DebugAll    DebugCategory = "all"
    DebugConfig DebugCategory = "config"  // Config loading/parsing
    DebugState  DebugCategory = "state"   // State file operations
    DebugNet    DebugCategory = "net"     // HTTP/API calls
    DebugGit    DebugCategory = "git"     // Git operations
    DebugFS     DebugCategory = "fs"      // File system operations
    DebugExec   DebugCategory = "exec"    // Command execution
)
```

**Usage:**
```bash
weg sync --debug=config,state    # Only config and state
weg remote push --debug=net      # Only network calls
weg sync --debug=all             # Everything (same as -vvv)
```

#### Debug Output Format

Debug output goes to **stderr** to keep stdout clean for piping:

```bash
# Normal output to stdout (pipeable)
$ weg site list --output json -vv 2>debug.log | jq '.[0]'

# Debug output to stderr
$ weg site list -vv
[DEBUG 14:23:01.123] config: Loading /home/user/project/.weg/weg.toml
[DEBUG 14:23:01.125] config: Parsed 3 sites, 5 apps
[DEBUG 14:23:01.126] state: Loading state.json (version: 1)
NAME              STATUS
mysite.localhost  Running
```

**Structured debug mode** (for tooling):
```bash
$ weg site list --debug --debug-format=json 2>&1 | jq -s '.'
[
  {"level":"debug","ts":"14:23:01.123","cat":"config","msg":"Loading","path":"/home/user/.weg/weg.toml"},
  {"level":"debug","ts":"14:23:01.125","cat":"config","msg":"Parsed","sites":3,"apps":5}
]
```

#### API Design Addition

```go
// ============================================================
// VERBOSITY & DEBUG (output.go)
// ============================================================

var (
    // Level is the current verbosity level
    Level Verbosity = VerbosityNormal

    // DebugCategories is the set of enabled debug categories
    // Empty means all categories when Level >= VerbosityDebug
    DebugCategories map[DebugCategory]bool

    // DebugFormat controls debug output format ("text" or "json")
    DebugFormat string = "text"

    // ShowTimestamps includes timestamps in debug output
    ShowTimestamps bool = true
)

// Verbose prints if Level >= VerbosityVerbose
func Verbose(message string)
func Verbosef(format string, args ...any)

// Debug prints if Level >= VerbosityDebug and category is enabled
func Debug(category DebugCategory, message string)
func Debugf(category DebugCategory, format string, args ...any)

// Trace prints if Level >= VerbosityTrace
func Trace(category DebugCategory, message string)
func Tracef(category DebugCategory, format string, args ...any)

// DebugEnabled returns true if debug output is enabled for category
func DebugEnabled(category DebugCategory) bool

// WithTiming wraps an operation with timing debug output
// Usage: defer output.WithTiming(DebugConfig, "parse weg.toml")()
func WithTiming(category DebugCategory, operation string) func()
```

#### Environment Variables

```bash
WEG_LOG_LEVEL=debug      # Set default verbosity (quiet|normal|verbose|debug|trace)
WEG_DEBUG=net,config     # Enable specific debug categories
WEG_DEBUG_FORMAT=json    # Structured debug output
WEG_NO_COLOR=1           # Disable colored output
```

#### Secret Redaction

Trace-level output that includes HTTP requests/responses or config values **must redact secrets** to prevent accidental leakage in logs or screenshots.

**Redacted patterns:**
- API keys and secrets
- Passwords and tokens
- Authorization headers
- Database credentials
- Any field matching: `password`, `secret`, `token`, `key`, `credential`, `auth`

**Implementation:**

```go
// Redactor handles secret masking in debug output
type Redactor struct {
    // Patterns to redact (compiled regexes)
    patterns []*regexp.Regexp

    // Known secret field names
    secretFields map[string]bool
}

var defaultRedactor = &Redactor{
    secretFields: map[string]bool{
        "password": true, "secret": true, "token": true,
        "api_key": true, "api_secret": true, "apikey": true,
        "auth": true, "authorization": true, "credential": true,
        "private_key": true, "access_token": true, "refresh_token": true,
    },
}

// Redact masks sensitive values in a string
func (r *Redactor) Redact(s string) string

// RedactMap masks sensitive values in a map (for JSON output)
func (r *Redactor) RedactMap(m map[string]any) map[string]any

// RedactHeaders masks sensitive HTTP headers
func (r *Redactor) RedactHeaders(h http.Header) http.Header
```

**Example output:**

```bash
# Trace output with redaction
[TRACE 14:23:01.200] net: POST https://site.frappe.cloud/api/method/login
[TRACE 14:23:01.200] net: Headers: {
  "Authorization": "token ***:***",
  "Content-Type": "application/json"
}
[TRACE 14:23:01.200] net: Body: {"usr":"admin","pwd":"***"}
[TRACE 14:23:01.350] net: Response: 200 OK (150ms)
[TRACE 14:23:01.350] net: Body: {"message":"Logged In","api_key":"***","api_secret":"***"}
```

**Config redaction:**

```bash
[DEBUG 14:23:01.100] config: Loaded credentials.toml
[DEBUG 14:23:01.100] config: api_key=abc***xyz, api_secret=***
```

**Rules:**
1. Show first 3 and last 3 chars for partial visibility: `abc***xyz`
2. If value < 8 chars, show only `***`
3. Never log full secrets, even in trace mode
4. Redaction cannot be disabled (security by design)

#### Example Usage in Code

```go
func loadConfig(path string) (*Config, error) {
    output.Verbosef("Loading config from %s", path)

    defer output.WithTiming(output.DebugConfig, "parse config")()

    data, err := os.ReadFile(path)
    if err != nil {
        output.Debugf(output.DebugFS, "ReadFile failed: %v", err)
        return nil, err
    }

    output.Debugf(output.DebugConfig, "Read %d bytes", len(data))

    var config Config
    if err := toml.Unmarshal(data, &config); err != nil {
        return nil, err
    }

    output.Debugf(output.DebugConfig, "Parsed: %d sites, %d apps",
        len(config.Sites), len(config.Apps))

    return &config, nil
}
```

**Output at different levels:**

```bash
# Normal (no flags)
$ weg sync
✓ Synced

# Verbose (-v)
$ weg sync -v
Loading config from .weg/weg.toml
Detecting changes...
No changes to apply
✓ Synced

# Debug (-vv)
$ weg sync -vv
[DEBUG 14:23:01.100] config: Loading .weg/weg.toml
[DEBUG 14:23:01.105] config: parse config took 5ms
[DEBUG 14:23:01.105] config: Parsed: 2 sites, 4 apps
[DEBUG 14:23:01.106] state: Loading state.json
[DEBUG 14:23:01.108] state: Version: 1, Apps: 4, Sites: 2
Loading config from .weg/weg.toml
Detecting changes...
[DEBUG 14:23:01.110] state: ComputeDiff took 2ms
[DEBUG 14:23:01.110] state: Diff: 0 additions, 0 removals
No changes to apply
✓ Synced
```

### 1.9 Acceptance Criteria

**Output Formatting:**
- [x] `--output` flag added to root command
- [x] Format auto-detection works (JSON when piped, table when TTY)
- [x] All tabwriter usage replaced with `output.List`
- [x] All symbol printing uses `output.Success/Error/Warning`
- [x] All JSON output uses unified formatter
- [x] Commands are scriptable with `--output json`

**Verbosity & Debug:**
- [x] `-v/-vv/-vvv` stacking works
- [x] `--log-level` explicit setting works
- [x] Precedence: `--log-level` > `-q` > `-v` count > env var
- [x] Debug categories work (`--debug-categories=net,config`)
- [x] Debug output goes to stderr (keeps stdout clean)
- [x] Environment variables respected (WEG_LOG_LEVEL, WEG_DEBUG)
- [x] `WithTiming` helper works

**Secret Redaction:**
- [x] Secrets redacted in trace output (passwords, tokens, keys)
- [x] HTTP Authorization headers redacted
- [x] Request/response bodies redacted
- [x] Config values with sensitive field names redacted
- [x] Partial visibility for long secrets (abc***xyz)
- [x] Redaction cannot be disabled

**Testing:**
- [x] Tests for all output functions and formats
- [x] Tests for redaction patterns
- [x] Tests for verbosity level precedence

### 1.3 Migration Path

**Files to update (8 tabwriter users):**
- `cmd/app/list.go`
- `cmd/cloud/benches.go`
- `cmd/cloud/marketplace.go`
- `cmd/cloud/sites.go`
- `cmd/cloud/status.go`
- `cmd/config/list_apps.go`
- `cmd/site/backup.go`
- `cmd/site/list.go`

**Before:**
```go
w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
fmt.Fprintln(w, "NAME\tSTATUS\tAPPS")
for _, s := range sites {
    fmt.Fprintf(w, "%s\t%s\t%s\n", s.Name, s.Status, s.Apps)
}
w.Flush()
```

**After:**
```go
t := output.NewTable("NAME", "STATUS", "APPS")
for _, s := range sites {
    t.Row(s.Name, s.Status, s.Apps)
}
t.Flush()
```

**Files to update (35+ symbol users):**
- All files using `fmt.Println("✓ ...")`
- Grep pattern: `fmt\.Print.*[✓✗⚠]`

**Before:**
```go
fmt.Println("✓ Connected")
fmt.Printf("✓ Cloned to %s/\n", dir)
```

**After:**
```go
output.Success("Connected")
output.Successf("Cloned to %s/", dir)
```

### 1.4 Acceptance Criteria

- [x] All tabwriter usage replaced with `output.Table`
- [x] All symbol printing uses `output.Success/Error/Warning`
- [x] All JSON output uses `output.JSON`
- [x] Verbose/Quiet flags wired to output package
- [x] Tests for all output functions

---

## Phase 2: Errors Package (`internal/errors`)

**Goal:** Centralize error handling with proper types and exit codes.

### 2.1 New Package Structure

```
internal/errors/
├── errors.go      # Core error types and constructors
├── codes.go       # Exit codes
├── print.go       # Error printing to stderr
└── errors_test.go # Tests
```

### 2.2 API Design

```go
package errors

import (
    "fmt"
    "os"
)

// ============================================================
// EXIT CODES (codes.go)
// ============================================================

const (
    ExitSuccess     = 0   // Successful execution
    ExitGeneric     = 1   // Generic error
    ExitUsage       = 2   // Invalid usage/arguments
    ExitConfig      = 3   // Configuration error
    ExitState       = 4   // State file error
    ExitNetwork     = 5   // Network/API error
    ExitNotFound    = 6   // Resource not found
    ExitPermission  = 7   // Permission denied
    ExitInterrupted = 130 // Interrupted (Ctrl+C)
)

// ExitCode returns the appropriate exit code for an error
func ExitCode(err error) int

// ============================================================
// ERROR TYPES (errors.go)
// ============================================================

// NotWegProject is returned when not in a weg-managed project
type NotWegProject struct {
    Path string
}

func (e *NotWegProject) Error() string {
    return "not a weg-managed project. Run 'weg init' first"
}

// ConfigError represents configuration file errors
type ConfigError struct {
    File    string
    Op      string // "read", "write", "parse"
    Err     error
}

func (e *ConfigError) Error() string
func (e *ConfigError) Unwrap() error

// StateError represents state file errors
type StateError struct {
    Op  string
    Err error
}

func (e *StateError) Error() string
func (e *StateError) Unwrap() error

// ValidationError represents input validation errors
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string

// APIError represents API/network errors
type APIError struct {
    StatusCode int
    Message    string
    Err        error
}

func (e *APIError) Error() string
func (e *APIError) Unwrap() error

// ============================================================
// CONSTRUCTORS (errors.go)
// ============================================================

// NotInProject returns a NotWegProject error
func NotInProject(path string) error

// Config returns a ConfigError
func Config(file, op string, err error) error

// State returns a StateError
func State(op string, err error) error

// Validation returns a ValidationError
func Validation(field, message string) error

// API returns an APIError
func API(statusCode int, message string, err error) error

// ============================================================
// PRINTING (print.go)
// ============================================================

// Print writes error to stderr with "Error: " prefix
func Print(err error)

// Printf writes formatted error to stderr
func Printf(format string, args ...interface{})

// Warn writes warning to stderr with "Warning: " prefix
func Warn(message string)

// Warnf writes formatted warning to stderr
func Warnf(format string, args ...interface{})

// Fatal prints error and exits with appropriate code
func Fatal(err error)

// ============================================================
// CHECKING (errors.go)
// ============================================================

// IsNotWegProject checks if error is NotWegProject
func IsNotWegProject(err error) bool

// IsConfig checks if error is ConfigError
func IsConfig(err error) bool

// IsValidation checks if error is ValidationError
func IsValidation(err error) bool
```

### 2.3 Migration Path

**Update `cmd/root.go` Execute function:**

**Before:**
```go
func Execute() {
    if err := rootCmd.Execute(); err != nil {
        os.Exit(1)
    }
}
```

**After:**
```go
func Execute() {
    if err := rootCmd.Execute(); err != nil {
        os.Exit(errors.ExitCode(err))
    }
}
```

**Update common error patterns:**

**Before:**
```go
return fmt.Errorf("not a weg-managed project")
return fmt.Errorf("failed to parse weg.toml: %w", err)
return fmt.Errorf("API error (HTTP %d): %s", code, msg)
```

**After:**
```go
return errors.NotInProject(path)
return errors.Config("weg.toml", "parse", err)
return errors.API(code, msg, nil)
```

### 2.4 Acceptance Criteria

- [x] All error types defined with proper Unwrap()
- [x] Exit codes mapped to error types
- [x] `cmd/root.go` uses `errors.ExitCode()`
- [x] High-frequency errors migrated to typed errors
- [x] `errors.Is()` checks work correctly
- [x] Tests for all error types and exit code mapping

---

## Phase 3: Prompt Package (`internal/prompt`)

**Goal:** Centralize all user interaction patterns.

### 3.1 New Package Structure

```
internal/prompt/
├── prompt.go      # Core prompt config
├── confirm.go     # Confirmation dialogs
├── password.go    # Secure password input
├── select.go      # Selection prompts (future)
└── prompt_test.go # Tests
```

### 3.2 API Design

```go
package prompt

import (
    "bufio"
    "os"
    "golang.org/x/term"
)

// ============================================================
// CONFIGURATION
// ============================================================

var (
    // AssumeYes skips all confirmation prompts
    AssumeYes bool

    // Reader for input (defaults to os.Stdin, can override for testing)
    Reader *bufio.Reader
)

// ============================================================
// CONFIRMATION (confirm.go)
// ============================================================

// Confirm asks a yes/no question, default No
// Returns true if user confirms
// Respects AssumeYes flag
//
// Example: Confirm("Delete site %s?", siteName)
// Output:  "Delete site mysite? [y/N]: "
func Confirm(format string, args ...interface{}) bool

// ConfirmDanger asks confirmation for destructive actions
// Includes "This cannot be undone." warning
// Respects AssumeYes flag
//
// Example: ConfirmDanger("Remove app %s?", appName)
// Output:  "Remove app myapp? This cannot be undone. [y/N]: "
func ConfirmDanger(format string, args ...interface{}) bool

// ConfirmDefault asks a yes/no question with specified default
// defaultYes=true makes Enter key confirm (Y/n)
// defaultYes=false makes Enter key cancel (y/N)
func ConfirmDefault(defaultYes bool, format string, args ...interface{}) bool

// ============================================================
// PASSWORD (password.go)
// ============================================================

// Password prompts for password with hidden input
// Falls back to plain input if not a terminal
//
// Example: Password("Enter admin password: ")
func Password(prompt string) (string, error)

// PasswordConfirm prompts for password with confirmation
// Returns error if passwords don't match
//
// Example: PasswordConfirm("New password for %s: ", username)
func PasswordConfirm(format string, args ...interface{}) (string, error)

// PasswordWithDefault prompts for password, returns default if empty
//
// Example: PasswordWithDefault("admin", "Admin password (default: admin): ")
func PasswordWithDefault(defaultVal, prompt string) (string, error)

// ============================================================
// TEXT INPUT (prompt.go)
// ============================================================

// String prompts for string input
//
// Example: String("Site name: ")
func String(prompt string) (string, error)

// StringDefault prompts for string, returns default if empty
//
// Example: StringDefault("localhost", "Hostname [localhost]: ")
func StringDefault(defaultVal, prompt string) (string, error)
```

### 3.3 Migration Path

**Files to update (19 confirmation users):**

| File | Current Pattern | Migration |
|------|-----------------|-----------|
| `cmd/app/remove.go` | Custom flags + bufio | `prompt.ConfirmDanger()` |
| `cmd/app/reinstall.go` | fmt.Scanln | `prompt.Confirm()` |
| `cmd/bench/drop.go` | bufio.Reader | `prompt.ConfirmDanger()` |
| `cmd/site/drop.go` | bufio.Reader | `prompt.ConfirmDanger()` |
| `cmd/site/new.go` | bufio.Reader | `prompt.PasswordWithDefault()` |
| `cmd/doc/delete.go` | fmt.Scanln | `prompt.ConfirmDanger()` |
| `cmd/log/clear.go` | bufio.Reader | `prompt.Confirm()` |
| `cmd/scheduler/purge.go` | fmt.Scanln | `prompt.ConfirmDanger()` |
| `cmd/upgrade.go` | fmt.Scanln | `prompt.Confirm()` |
| `cmd/sync_helpers.go` | bufio.Reader | `prompt.Confirm()` |
| `cmd/remote/clone.go` | bufio.Reader | `prompt.ConfirmDefault(true, ...)` |
| `cmd/remote/login.go` | bufio.Reader | `prompt.String()` |
| `cmd/cloud/login.go` | bufio.Reader | `prompt.String()` |
| `cmd/site/password.go` | term.ReadPassword | `prompt.PasswordConfirm()` |
| `cmd/user/password.go` | term.ReadPassword | `prompt.PasswordConfirm()` |
| (+ 4 more) | Various | Various |

**Before (`cmd/app/remove.go`):**
```go
var forceRemove, yesRemove bool

removeCmd.Flags().BoolVarP(&forceRemove, "force", "f", false, "Force removal")
removeCmd.Flags().BoolVarP(&yesRemove, "yes", "y", false, "Skip confirmation")

// In RunE:
if !forceRemove && !yesRemove {
    fmt.Printf("Remove app %s? This cannot be undone. [y/N]: ", appName)
    reader := bufio.NewReader(os.Stdin)
    answer, _ := reader.ReadString('\n')
    answer = strings.TrimSpace(strings.ToLower(answer))
    if answer != "y" && answer != "yes" {
        fmt.Println("Cancelled.")
        return nil
    }
}
```

**After:**
```go
// Remove duplicate flags, use only --force
var forceRemove bool
removeCmd.Flags().BoolVarP(&forceRemove, "force", "f", false, "Skip confirmation")

// In RunE:
if !forceRemove && !prompt.ConfirmDanger("Remove app %s?", appName) {
    output.Print("Cancelled.")
    return nil
}
```

### 3.4 Flag Standardization

As part of this phase, standardize confirmation flags:

| Current | Standardized | Meaning |
|---------|--------------|---------|
| `--force, -f` (some cmds) | `--force, -f` | Skip confirmation |
| `--yes, -y` (some cmds) | Remove | Redundant |
| `--yes, -y` (global) | Keep | Global assume-yes |

**Rule:** Commands use `--force/-f` for local skip, global `--yes/-y` in `prompt.AssumeYes`.

### 3.5 Acceptance Criteria

- [x] All 19 confirmation files migrated to `prompt.Confirm*`
- [x] All 3 password files migrated to `prompt.Password*`
- [x] Duplicate `--force`/`--yes` flags consolidated
- [x] `prompt.AssumeYes` wired to global `--yes` flag
- [x] Terminal detection works (masked input)
- [x] Non-terminal fallback works (piped input)
- [x] Tests for all prompt functions

---

## Phase 4: Flag Conventions

**Goal:** Resolve flag conflicts and document standards.

### 4.1 Flag Conflict Resolution

**Problem:** `-f` means different things:

| Command | `-f` means |
|---------|------------|
| `app remove` | `--force` |
| `api get` | `--filters` |
| `doc list` | `--filters` |

**Resolution:**

```
--force     → Long form only (no -f shorthand)
--filters   → -F (capital F)
--fields    → Keep as-is (no conflict)
```

**Files to update:**
- `cmd/app/remove.go` - Remove `-f`, keep `--force`
- `cmd/api/get.go` - Change `-f` to `-F` for filters
- `cmd/doc/list.go` - Change `-f` to `-F` for filters

### 4.2 Flag Naming Standards

Document in `docs/CLI_CONVENTIONS.md`:

```markdown
## Flag Naming Standards

### Global Flags (defined in root.go)

**Output Control:**
- `--output, -o` - Output format: auto, json, table, plain, quiet
- `--quiet, -q` - Suppress non-essential output (errors only)
- `--verbose, -v` - Increase verbosity (stackable: -v, -vv, -vvv)
- `--log-level` - Set verbosity explicitly: quiet, normal, verbose, debug, trace
- `--debug-categories` - Filter debug output: all,config,state,net,git,fs,exec

**Behavior:**
- `--yes, -y` - Assume yes for all prompts
- `--config` - Path to config file
- `--chdir, -C` - Run as if started in path

### Verbosity Levels

Two equivalent ways to set verbosity - use whichever fits:

| Quick | Explicit | Level | What You See |
|-------|----------|-------|--------------|
| `-q` | `--log-level=quiet` | Quiet | Errors and final result only |
| (default) | `--log-level=normal` | Normal | Standard operation output |
| `-v` | `--log-level=verbose` | Verbose | + What's happening ("Loading...", "Connecting...") |
| `-vv` | `--log-level=debug` | Debug | + Internal state, timing, decisions |
| `-vvv` | `--log-level=trace` | Trace | + HTTP requests, file ops (secrets redacted) |

**Precedence:** `--log-level` > `-q` > `-v` count > `WEG_LOG_LEVEL` env var

### Environment Variables

| Variable | Purpose | Example |
|----------|---------|---------|
| `WEG_LOG_LEVEL` | Default verbosity | `debug`, `trace`, `quiet` |
| `WEG_DEBUG` | Debug categories | `net,config` |
| `WEG_DEBUG_FORMAT` | Debug output format | `json` for structured |
| `WEG_NO_COLOR` or `NO_COLOR` | Disable colors | `1` |
| `WEG_API_KEY` | API key | (credential) |
| `WEG_API_SECRET` | API secret | (credential) |

### Common Flags (use consistently across commands)
- `--site, -s` - Site name
- `--force` - Skip confirmation (NO shorthand to avoid conflict)
- `--dry-run, -n` - Preview without executing
- `--filters, -F` - Filter expression (capital F)
- `--fields` - Fields to return
- `--limit, -l` - Limit results

### Credential Flags (prefer env vars)
- `--api-key` - API key (prefer WEG_API_KEY env)
- `--api-secret` - API secret (prefer WEG_API_SECRET env)
- `--password` - Password (prefer interactive prompt)

### Destructive Operation Flags
- Always require `--force` OR global `--yes` to skip confirmation
- Never default to yes for destructive actions
```

### 4.3 Acceptance Criteria

- [x] `-f` conflict resolved (--force long only, -F for filters)
- [x] `docs/CLI_CONVENTIONS.md` created
- [x] All flag inconsistencies documented and fixed
- [ ] PR template updated to check flag conventions

---

## Phase 5: Apply Patterns to Commands

**Goal:** Update all commands to use new packages.

### 5.1 Command Classification

Commands fall into two categories based on their output:

**Data Commands** (use `output.List`/`output.Item` - support `--output` flag):
- List commands: `site list`, `app list`, `user list`, `log list`, `scheduler jobs`
- Show commands: `site config get`, `doc show`, `doctype show`, `cloud status`
- Query commands: `db query`, `api get`

**Action Commands** (use `output.Success`/`output.Step` - status only):
- Create/Delete: `site new`, `site drop`, `app get`, `app remove`
- Operations: `sync`, `start`, `stop`, `restart`, `build`
- Remote: `remote push`, `remote pull`, `remote clone`

### 5.2 Command Update Checklist

**For Data Commands:**
```
[ ] Define output struct with json tags
[ ] Replace tabwriter/manual output with output.List or output.Item
[ ] Ensure --output json produces valid, parseable JSON
[ ] Test with: cmd --output json | jq '.'
```

**For Action Commands:**
```
[ ] Replace symbol printing with output.Success/Error/Warning
[ ] Replace confirmations with prompt.Confirm*
[ ] Use output.Step for multi-step operations
[ ] Ensure status messages are consistent
```

**For All Commands:**
```
[ ] Import internal/output, internal/errors, internal/prompt as needed
[ ] Replace fmt.Errorf with typed errors where appropriate
[ ] Update flags per conventions
[ ] Ensure proper exit codes returned
```

### 5.3 Priority Order

**Tier 1 - High traffic commands (update first):**
- `cmd/site/list.go` (data)
- `cmd/app/list.go` (data)
- `cmd/sync.go` (action)
- `cmd/start.go` / `cmd/stop.go` (action)
- `cmd/site/new.go` / `cmd/site/drop.go` (action)
- `cmd/app/get.go` / `cmd/app/remove.go` (action)

**Tier 2 - Remote/Cloud commands:**
- `cmd/remote/*.go` (8 files - mostly action)
- `cmd/cloud/*.go` (10 files - mix of data and action)

**Tier 3 - Other commands:**
- Remaining `cmd/` files

### 5.4 Example Migrations

**Data Command Migration (site list):**

```go
// Before
func listSites(cmd *cobra.Command, args []string) error {
    sites, err := loadSites()
    if err != nil {
        return fmt.Errorf("failed to load sites: %w", err)
    }
    w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
    fmt.Fprintln(w, "NAME\tSTATUS\tAPPS")
    for _, s := range sites {
        fmt.Fprintf(w, "%s\t%s\t%d\n", s.Name, s.Status, len(s.Apps))
    }
    w.Flush()
    return nil
}

// After
type SiteInfo struct {
    Name   string `json:"name"`
    Status string `json:"status"`
    Apps   int    `json:"apps"`
}

func listSites(cmd *cobra.Command, args []string) error {
    sites, err := loadSites()
    if err != nil {
        return errors.State("load", err)
    }

    var items []SiteInfo
    for _, s := range sites {
        items = append(items, SiteInfo{
            Name:   s.Name,
            Status: s.Status,
            Apps:   len(s.Apps),
        })
    }
    return output.List(items)
}
```

**Action Command Migration (app remove):**

```go
// Before
func removeApp(cmd *cobra.Command, args []string) error {
    appName := args[0]
    if !forceRemove && !yesRemove {
        fmt.Printf("Remove app %s? This cannot be undone. [y/N]: ", appName)
        reader := bufio.NewReader(os.Stdin)
        answer, _ := reader.ReadString('\n')
        if strings.TrimSpace(strings.ToLower(answer)) != "y" {
            fmt.Println("Cancelled.")
            return nil
        }
    }
    if err := doRemove(appName); err != nil {
        return fmt.Errorf("failed to remove app: %w", err)
    }
    fmt.Printf("✓ App %s removed\n", appName)
    return nil
}

// After
func removeApp(cmd *cobra.Command, args []string) error {
    appName := args[0]

    if !forceRemove && !prompt.ConfirmDanger("Remove app %s?", appName) {
        output.Print("Cancelled.")
        return nil
    }

    if err := doRemove(appName); err != nil {
        return errors.Config(appName, "remove", err)
    }

    output.Successf("App %s removed", appName)
    return nil
}
```

### 5.5 Acceptance Criteria

- [x] All Tier 1 commands updated
- [x] All Tier 2 commands updated
- [x] All Tier 3 commands updated
- [x] No direct tabwriter imports remain
- [x] No direct symbol printing remains
- [x] All confirmations use prompt package
- [x] All data commands support `--output json`
- [x] Tests pass
- [x] `weg site list --output json | jq '.'` works

---

## Phase 6: Logical Inconsistencies

**Goal:** Fix behavioral inconsistencies (after design system is stable).

### 6.1 Issues to Address

| Issue | Files | Fix |
|-------|-------|-----|
| Push/pull commit message asymmetry | `remote/push.go`, `remote/pull.go` | Pull should also support `-m` |
| Missing push deletion confirmation | `remote/push.go` | Add `prompt.ConfirmDanger()` for deletions |
| Missing deploy confirmation | `cloud/deploy.go` | Add `prompt.Confirm()` |
| Partial failure exit codes | `remote/sync.go`, others | Return non-zero on partial failure |
| State save error inconsistency | Various | Standardize to warn + continue |
| Config set not implemented | `config/set.go` | Implement or remove |
| Cloud deploy incomplete | `cloud/deploy.go` | Add site selection |

### 6.2 State Management Issues

| Issue | Location | Fix |
|-------|----------|-----|
| No file locking | `internal/state/state.go` | Add flock |
| No state validation | `internal/state/state.go` | Add `ValidateState()` |
| Silent config parse failures | `internal/config/pyproject.go` | Return error list |
| No config versioning | `internal/config/wegtoml.go` | Add version field |

### 6.3 Acceptance Criteria

- [x] Push/pull symmetry fixed
- [x] All destructive ops have confirmation
- [x] Partial failures return non-zero
- [x] State save errors handled consistently
- [x] Incomplete commands completed or removed
- [x] State management issues fixed

---

## Implementation Timeline

```
Phase 1: Output Package         ████████████████████ ✓ Complete
Phase 2: Errors Package         ████████████████████ ✓ Complete
Phase 3: Prompt Package         ████████████████████ ✓ Complete
Phase 4: Flag Conventions       ████████████████████ ✓ Complete
Phase 5: Apply to Commands      ████████████████████ ✓ Complete
Phase 6: Logical Fixes          ████████████████████ ✓ Complete
```

### Dependencies

```
Phase 1 (Output) ──┬──→ Phase 5 (Commands)
Phase 2 (Errors) ──┤
Phase 3 (Prompt) ──┤
Phase 4 (Flags)  ──┘
                          ↓
                   Phase 6 (Logical)
```

Phases 1-4 can be developed in parallel but must all complete before Phase 5.
Phase 6 depends on Phase 5.

---

## Testing Strategy

### Unit Tests

Each new package gets comprehensive unit tests:

```go
// internal/output/output_test.go
func TestTable_Basic(t *testing.T)
func TestTable_EmptyRows(t *testing.T)
func TestSuccess_Output(t *testing.T)
func TestJSON_Formatting(t *testing.T)

// internal/errors/errors_test.go
func TestExitCode_NotWegProject(t *testing.T)
func TestExitCode_ConfigError(t *testing.T)
func TestConfigError_Unwrap(t *testing.T)
func TestIsNotWegProject(t *testing.T)

// internal/prompt/prompt_test.go
func TestConfirm_Yes(t *testing.T)
func TestConfirm_No(t *testing.T)
func TestConfirm_AssumeYes(t *testing.T)
func TestPassword_Terminal(t *testing.T)
func TestPassword_Piped(t *testing.T)
```

### Integration Tests

Test common workflows end-to-end:

```go
func TestSiteCreate_OutputFormat(t *testing.T)
func TestAppRemove_Confirmation(t *testing.T)
func TestRemotePush_PartialFailure(t *testing.T)
```

---

## Rollback Plan

Each phase is independently deployable. If issues arise:

1. **Phase 1-4:** Revert package, commands still work with old patterns
2. **Phase 5:** Revert individual command changes
3. **Phase 6:** Revert behavioral changes independently

---

## Success Metrics

| Metric | Before | Target | Achieved |
|--------|--------|--------|----------|
| Tabwriter imports in cmd/ | 8 | 0 | ✓ 0 |
| Direct symbol printing | 35+ | 0 | ✓ 0 |
| Confirmation implementations | 19 (2 patterns) | 19 (1 pattern) | ✓ 1 pattern |
| Exit codes used | 1 | 7+ | ✓ 7 |
| Flag conflicts | 1 (`-f`) | 0 | ✓ 0 |
| Test coverage (output pkg) | 0% | 80%+ | ✓ 76.2% |
| Test coverage (workspace pkg) | 0% | 20%+ | ✓ 21.5% |
| Commands supporting `--output json` | 0 | All list/show | ✓ All |
| Commands scriptable (JSON output) | ~5 | All data-returning | ✓ All |

---

## Appendix: File Impact Summary

### New Files (to create)

```
internal/output/output.go
internal/output/table.go
internal/output/symbols.go
internal/output/json.go
internal/output/output_test.go
internal/errors/errors.go
internal/errors/codes.go
internal/errors/print.go
internal/errors/errors_test.go
internal/prompt/prompt.go
internal/prompt/confirm.go
internal/prompt/password.go
internal/prompt/prompt_test.go
docs/CLI_CONVENTIONS.md
```

### Files to Modify

**Phase 1 (Output):** 8 tabwriter + 35 symbol = ~40 files
**Phase 2 (Errors):** `cmd/root.go` + high-frequency error sites = ~20 files
**Phase 3 (Prompt):** 19 confirmation + 3 password = ~20 files
**Phase 4 (Flags):** 3 files with `-f` conflict
**Phase 5 (Commands):** All 155 cmd/ files (varying degree)
**Phase 6 (Logical):** ~15 files with behavioral issues

**Total unique files affected:** ~60-80 files (some overlap)

---

## Appendix B: Command Classification

### Data Commands (support `--output json/table/plain`)

These commands return structured data that users may want to script against:

| Command | Output Type | Fields |
|---------|-------------|--------|
| `weg site list` | List | name, status, apps |
| `weg site config get` | Item/List | key, value |
| `weg app list` | List | name, version, branch |
| `weg user list` | List | email, enabled, roles |
| `weg log list` | List | method, status, time |
| `weg scheduler jobs` | List | job, queue, status |
| `weg db query` | List | (dynamic from SQL) |
| `weg api get` | Item/List | (dynamic from API) |
| `weg doc list` | List | name, modified |
| `weg doc show` | Item | (doctype fields) |
| `weg doctype list` | List | name, module |
| `weg doctype show` | Item | (doctype meta) |
| `weg bench list` | List | name, path |
| `weg cloud sites` | List | name, status, plan |
| `weg cloud benches` | List | name, version |
| `weg cloud status` | Item | site, status, jobs |
| `weg remote status` | Item | site, changes |
| `weg status` | Item | services, state |

### Action Commands (status output only)

These commands perform actions and output status/progress:

| Command | Output Pattern |
|---------|---------------|
| `weg init` | Steps + Success |
| `weg sync` | Steps + Success |
| `weg start` / `weg stop` | Success/Error |
| `weg restart` | Success |
| `weg build` | Steps + Success |
| `weg site new` | Steps + Success |
| `weg site drop` | Confirm + Success |
| `weg site install` | Steps + Success |
| `weg site backup` / `restore` | Steps + Success |
| `weg app get` | Steps + Success |
| `weg app remove` | Confirm + Success |
| `weg user create` | Success |
| `weg user password` | Password prompt + Success |
| `weg remote clone` | Steps + Success |
| `weg remote push` / `pull` | Steps + Summary |
| `weg remote sync` | Steps + Summary |
| `weg remote login` / `logout` | Success |
| `weg cloud login` / `logout` | Success |
| `weg cloud deploy` | Confirm + Success |
| `weg docker up` / `down` | Success |
| `weg upgrade` | Confirm + Steps + Success |

### Hybrid Commands

Some commands can be either, depending on flags:

| Command | Default | With Flag |
|---------|---------|-----------|
| `weg site backup` | Action (creates backup) | Data with `--list` |
| `weg config get` | Data | - |
| `weg config set` | Action | - |
