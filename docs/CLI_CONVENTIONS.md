# CLI Conventions

This document defines the conventions for the weg CLI to ensure consistency across all commands.

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
| `WEG_NO_COLOR` or `NO_COLOR` | Disable colors | `1` |
| `WEG_API_KEY` | API key | (credential) |
| `WEG_API_SECRET` | API secret | (credential) |

### Common Flags (use consistently across commands)

| Flag | Shorthand | Purpose |
|------|-----------|---------|
| `--site` | `-s` | Site name |
| `--force` | (none) | Skip confirmation (no shorthand to avoid conflicts) |
| `--dry-run` | `-n` | Preview without executing |
| `--filters` | `-F` | Filter expression (capital F) |
| `--fields` | (none) | Fields to return |
| `--limit` | `-l` | Limit results |

### Credential Flags (prefer env vars or prompts)

- `--api-key` - API key (prefer `WEG_API_KEY` env var)
- `--api-secret` - API secret (prefer `WEG_API_SECRET` env var)
- `--password` - Password (prefer interactive prompt)

## Output Formats

Commands that return data support multiple output formats via `--output`:

| Format | Use Case | Example |
|--------|----------|---------|
| `auto` | Default: table for TTY, JSON when piped | `weg site list` |
| `json` | Scripting and automation | `weg site list -o json \| jq .` |
| `table` | Human-readable with headers | `weg site list -o table` |
| `plain` | Simple text, easy to parse | `weg site list -o plain` |
| `quiet` | Minimal output (IDs/names only) | `weg site list -o quiet` |

## Destructive Operations

Commands that modify or delete data:

1. **Always require confirmation** for destructive actions (drop, remove, delete)
2. **Skip with `--force`** or global `--yes` flag
3. **Never default to yes** for destructive actions
4. **Show what will be affected** before asking for confirmation

Example:
```bash
# Requires confirmation
weg site drop mysite

# Skip confirmation with --force
weg site drop mysite --force

# Skip all confirmations with global -y
weg -y site drop mysite
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Generic error |
| 2 | Invalid usage/arguments |
| 3 | Configuration error |
| 4 | State file error |
| 5 | Network/API error |
| 6 | Resource not found |
| 7 | Permission denied |
| 130 | Interrupted (Ctrl+C) |

## Error Messages

- Print errors to stderr
- Include the error symbol prefix (✗)
- Provide actionable hints when possible
- Redact secrets in all output (including debug)

Example:
```
✗ site "mysite" not found
Hint: Run 'weg site list' to see available sites
```

## Command Structure

### Naming
- Use lowercase, kebab-case for multi-word commands: `weg site backup-create`
- Group related commands under parent: `weg site`, `weg app`, `weg remote`
- Use consistent verbs: `list`, `show`, `create`, `delete`, `update`

### Arguments vs Flags
- Required positional: The main thing being operated on (e.g., site name)
- Optional positional: Additional targets
- Flags: Modifiers, options, and filters

```bash
# Good: site name is the main target
weg site drop mysite --force

# Good: app name is optional (defaults to current)
weg app install erpnext --branch main
```

## Status Messages

Use consistent symbols for status output:

| Symbol | Meaning | Function |
|--------|---------|----------|
| ✓ | Success | `output.Success()` |
| ✗ | Error | `output.Error()` |
| ⚠ | Warning | `output.Warning()` |
| → | Info | `output.Info()` |
| [n/m] | Progress | `output.Step()` |

## Confirmation Prompts

Use the prompt package for consistent confirmations:

```go
// Standard confirmation (default: no)
if !prompt.Confirm("Delete site %s?", siteName) {
    return nil
}

// Dangerous action (includes warning)
if !prompt.ConfirmDanger("Drop database for %s?", siteName) {
    return nil
}

// Default yes (for likely-desired actions)
if !prompt.ConfirmDefault(true, "Install dependencies?") {
    return nil
}
```
