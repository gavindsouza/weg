# Weg Product Roadmap

## Executive Summary

Weg is a modern CLI replacement for Frappe's `bench` tool, offering declarative configuration, faster tooling (devbox, uv, process-compose), and an app-centric development model. This document outlines the path to market readiness.

### Current State

| Metric | Value | Target |
|--------|-------|--------|
| Commands implemented | 54+ | ~70 |
| Test coverage | ~20% | 80%+ |
| Critical missing features | 4 | 0 |
| Known TODOs/bugs | 3 | 0 |
| Documentation completeness | 60% | 100% |

### Key Differentiators from Bench

1. **Declarative config** - `weg.toml` / `pyproject.toml [tool.weg]` vs imperative commands
2. **App-centric mode** - Your app is the project root, bench hidden in `.weg/`
3. **Modern tooling** - devbox (Nix), uv (fast Python), process-compose
4. **Direct API access** - `weg api` without HTTP overhead
5. **Multi-version testing** - `weg test --versions 14,15,16`
6. **Cloud-native** - Built-in Frappe Cloud, Docker, container image support

---

## Gap Analysis

### Critical Missing Features (Blockers for Launch)

| Feature | Bench Equivalent | Priority | Effort |
|---------|------------------|----------|--------|
| Backup | `bench backup` | P0 | Medium |
| Restore | `bench restore` | P0 | Medium |
| Clear cache | `bench clear-cache` | P0 | Low |
| Set admin password | `bench set-admin-password` | P0 | Low |

**Why these are blockers:**
- Developers use backup/restore multiple times daily
- Cache clearing is the #1 troubleshooting step
- Password reset is needed after every restore from production

### High Priority Missing Features

| Feature | Bench Equivalent | Priority | Effort |
|---------|------------------|----------|--------|
| Scheduler control | `bench enable/disable-scheduler` | P1 | Low |
| Job management | `bench show-pending-jobs`, `bench purge-jobs` | P1 | Low |
| Export fixtures | `bench export-fixtures` | P1 | Medium |
| Version display | `bench version` | P1 | Low |
| Add to hosts | `bench add-to-hosts` | P1 | Low |

### Quality Gaps

| Issue | Severity | Files Affected |
|-------|----------|----------------|
| No tests for state package | High | internal/state/*.go |
| No tests for services package | High | internal/services/*.go |
| No tests for cloud package | High | internal/cloud/*.go |
| No tests for apps package | Medium | internal/apps/*.go |
| No tests for container package | Medium | internal/container/*.go |
| No atomic file writes | High | All config/state writes |
| Database not dropped on app remove | High | cmd/sync.go:891 |
| No structured error types | Medium | All packages |

### Known TODOs in Codebase

1. `cmd/sync.go:891` - **HIGH**: Database not dropped when removing app
2. `cmd/update.go:193` - **MEDIUM**: Parallel esbuild not implemented
3. `cmd/start.go:102` - **LOW**: Sync not called automatically on start

---

## Implementation Phases

### Phase 0: Foundation (Week 1-2)

**Goal:** Fix critical quality issues before adding features

| Task | Priority | Effort |
|------|----------|--------|
| Add tests for `internal/state` | P0 | 1 day |
| Add tests for `internal/services` | P0 | 1 day |
| Implement atomic file writes | P0 | 1 day |
| Fix database drop on app remove | P0 | 0.5 day |
| Add structured error types | P1 | 1 day |
| Add signal handling to all commands | P1 | 0.5 day |

**Exit criteria:**
- All internal packages have >70% test coverage
- No data corruption possible on crash
- Graceful shutdown works everywhere

### Phase 1: Essential Developer Tools (Week 3-4)

**Goal:** Implement the 4 critical missing features

```
weg backup [site]                    # Backup database + files
weg backup --all                     # Backup all sites
weg restore <backup-file> [site]     # Restore from backup
weg cache clear [--site]             # Clear Redis + local cache
weg password [email] [--site]        # Set/reset user password
```

**Implementation details:**

#### `weg backup`
- Create timestamped backup: `{site}_{datetime}.sql.gz`
- Include private files: `{site}_{datetime}_files.tar.gz`
- Store in `.weg/backups/` or custom location
- Support `--output` flag for custom path

#### `weg restore`
- Detect backup format (SQL, gzipped SQL, with/without files)
- Create site if doesn't exist
- Optionally restore files
- Clear cache after restore

#### `weg cache clear`
- Clear Redis cache (all keys for site)
- Clear `__pycache__` directories
- Clear `.pyc` files
- Clear `node_modules/.cache`

#### `weg password`
- Direct database update (no frappe boot needed)
- Hash password correctly
- Clear sessions after change

**Exit criteria:**
- All 4 commands implemented and tested
- Integration tests pass
- Documentation complete

### Phase 2: Scheduler & Jobs (Week 5)

**Goal:** Full control over background processing

```
weg scheduler status                 # Show scheduler state
weg scheduler enable [--site]        # Enable scheduler
weg scheduler disable [--site]       # Disable scheduler

weg jobs list [--site]               # Show pending/failed jobs
weg jobs retry <job-id>              # Retry failed job
weg jobs purge [--failed] [--site]   # Clear job queue
```

**Exit criteria:**
- Scheduler can be controlled per-site
- Job visibility matches bench
- Failed jobs can be diagnosed

### Phase 3: Data Management (Week 6)

**Goal:** Data import/export for development workflows

```
weg fixtures export [--app] [--doctype]   # Export to JSON
weg fixtures import <file>                 # Import fixtures

weg data import <file> [--doctype]        # Import CSV/XLSX
weg data export <doctype> [--format]      # Export to CSV/JSON

weg version                                # Show all versions
weg version --json                         # Machine-readable
```

**Exit criteria:**
- Fixtures round-trip correctly
- Data import handles errors gracefully
- Version info matches `bench version`

### Phase 4: Quality of Life (Week 7)

**Goal:** Polish and convenience features

```
weg hosts add [--site]               # Add to /etc/hosts (sudo)
weg hosts remove [--site]            # Remove from /etc/hosts

weg reinstall [site]                 # Drop and recreate site
weg execute <method> [args...]       # Run Python method

weg shell                            # Enter devbox shell
weg pip <args>                       # Run pip in venv
```

**Exit criteria:**
- All common bench commands have weg equivalents
- No workflow requires falling back to bench

### Phase 5: Testing & Documentation (Week 8)

**Goal:** Production-ready quality

| Task | Target |
|------|--------|
| Unit test coverage | >80% |
| Integration tests | All core workflows |
| CLI help text | Complete for all commands |
| USAGE.md | All commands documented |
| Error messages | Actionable, with suggestions |
| Man pages | Generated from help text |

**Exit criteria:**
- CI passes on all supported platforms
- New contributor can onboard in <30 min
- All error messages suggest fixes

---

## Architecture Decisions

### File Write Safety

**Current:** Direct writes to config/state files
**Problem:** Crash during write = corrupted file
**Solution:** Atomic write pattern

```go
func atomicWrite(path string, data []byte) error {
    tmp := path + ".tmp"
    if err := os.WriteFile(tmp, data, 0644); err != nil {
        return err
    }
    return os.Rename(tmp, path)  // Atomic on POSIX
}
```

### Structured Errors

**Current:** String errors with `fmt.Errorf`
**Problem:** Can't programmatically handle specific errors
**Solution:** Error types

```go
type AppNotFoundError struct {
    Name string
}

type SiteNotFoundError struct {
    Name string
}

type ConfigError struct {
    Path    string
    Message string
}
```

### Backup Format

**Proposed structure:**
```
backups/
  mysite.localhost_2024-01-15_143022/
    database.sql.gz
    private_files.tar.gz
    public_files.tar.gz
    manifest.json  # metadata: frappe version, apps, etc.
```

---

## Success Metrics

### Functional Completeness

| Metric | Current | Target |
|--------|---------|--------|
| Bench command coverage | 65% | 95% |
| Core workflows without fallback | 70% | 100% |
| Error recovery documented | 20% | 100% |

### Quality

| Metric | Current | Target |
|--------|---------|--------|
| Test coverage | 20% | 80% |
| CI pass rate | - | 99% |
| P0 bugs | 1 | 0 |
| P1 bugs | 2 | 0 |

### Developer Experience

| Metric | Current | Target |
|--------|---------|--------|
| Time to first `weg start` | ~10 min | <5 min |
| Commands with `--help` | 80% | 100% |
| Error messages with suggestions | 30% | 90% |

---

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Frappe version incompatibility | Medium | High | Test against 14, 15, 16, develop |
| devbox installation friction | Medium | Medium | Provide `weg self install-tools` |
| Cloud API changes | Low | Medium | Version API client, handle gracefully |
| Data loss from bugs | Low | Critical | Atomic writes, backup before destructive ops |

---

## Open Questions

1. **Should `weg sync` auto-run on `weg start`?** - Current TODO suggests yes
2. **Support for bench plugins?** - Some orgs have custom bench commands
3. **Windows support?** - devbox works on WSL, native Windows unclear
4. **Offline mode?** - Cache dependencies for air-gapped development?

---

## Appendix: Command Mapping

### Implemented (54 commands)

| Weg | Bench |
|-----|-------|
| `weg new` | `bench new-app` |
| `weg init` | `bench init` |
| `weg sync` | (no equivalent - declarative) |
| `weg start` | `bench start` |
| `weg stop` | (Ctrl+C) |
| `weg build` | `bench build` |
| `weg test` | `bench run-tests` |
| `weg console` | `bench console` |
| `weg mariadb` | `bench mariadb` |
| `weg browse` | `bench browse` |
| `weg app get` | `bench get-app` |
| `weg app remove` | `bench remove-app` |
| `weg site new` | `bench new-site` |
| `weg site drop` | `bench drop-site` |
| `weg site use` | `bench use` |
| `weg site install` | `bench install-app` |
| `weg update` | `bench update` |
| `weg exec` | `bench execute` |
| `weg api *` | (no equivalent - new feature) |
| `weg cloud *` | (no equivalent - new feature) |
| `weg docker *` | (no equivalent - new feature) |
| `weg image *` | (no equivalent - new feature) |

### Not Yet Implemented (16 critical commands)

| Bench | Proposed Weg | Priority |
|-------|--------------|----------|
| `bench backup` | `weg backup` | P0 |
| `bench restore` | `weg restore` | P0 |
| `bench clear-cache` | `weg cache clear` | P0 |
| `bench set-admin-password` | `weg password` | P0 |
| `bench enable-scheduler` | `weg scheduler enable` | P1 |
| `bench disable-scheduler` | `weg scheduler disable` | P1 |
| `bench show-pending-jobs` | `weg jobs list` | P1 |
| `bench purge-jobs` | `weg jobs purge` | P1 |
| `bench export-fixtures` | `weg fixtures export` | P1 |
| `bench version` | `weg version` | P1 |
| `bench add-to-hosts` | `weg hosts add` | P2 |
| `bench reinstall` | `weg reinstall` | P2 |
| `bench data-import` | `weg data import` | P2 |
| `bench add-user` | `weg user add` | P3 |
| `bench disable-user` | `weg user disable` | P3 |
| `bench destroy-all-sessions` | `weg sessions clear` | P3 |
