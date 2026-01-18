# Weg Product Roadmap

## Executive Summary

Weg is a modern CLI replacement for Frappe's `bench` tool, offering declarative configuration, faster tooling (devbox, uv, process-compose), and three distinct development modes:

1. **App-centric** - Your app is the project root, bench hidden in `.weg/`
2. **Bench-centric** - Traditional bench directory structure
3. **Remote-site** - Work with remote Frappe sites without direct bench access

### Current State

| Metric | Value | Target | Status |
|--------|-------|--------|--------|
| Commands implemented | 70+ | ~70 | Complete |
| Test coverage | ~45% | 80%+ | In progress |
| Critical missing features | 0 | 0 | Complete |
| Known TODOs/bugs | 0 | 0 | Complete |
| Documentation completeness | 60% | 100% | In progress |

### Key Differentiators from Bench

1. **Declarative config** - `weg.toml` / `pyproject.toml [tool.weg]` vs imperative commands
2. **Three development modes** - App-centric, bench-centric, and remote-site
3. **Modern tooling** - devbox (Nix), uv (fast Python), process-compose
4. **Direct API access** - `weg api` without HTTP overhead
5. **Remote site editing** - Git-backed local editing of remote site customizations
6. **Cloud-native** - Built-in Frappe Cloud, Docker, container image support

---

## Feature Status

### Core Commands (Complete)

| Command | Description | Status |
|---------|-------------|--------|
| `weg new` | Create new Frappe app | Done |
| `weg init` | Initialize weg in directory | Done |
| `weg sync` | Apply configuration changes | Done |
| `weg start` | Start development servers | Done |
| `weg stop` | Stop development services | Done |
| `weg build` | Build assets and frontend | Done |
| `weg test` | Run tests | Done |
| `weg update` | Update apps to latest | Done |
| `weg version` | Show version info | Done |

### Site Management (Complete)

| Command | Description | Status |
|---------|-------------|--------|
| `weg site new` | Create new site | Done |
| `weg site drop` | Delete site | Done |
| `weg site list` | List all sites | Done |
| `weg site use` | Set default site | Done |
| `weg site install` | Install app on site | Done |
| `weg site backup` | Backup database + files | Done |
| `weg site restore` | Restore from backup | Done |
| `weg site password` | Set/reset user password | Done |
| `weg site browse` | Open site in browser | Done |
| `weg site config` | Manage site config | Done |
| `weg site hosts` | Manage /etc/hosts entries | Done |
| `weg site maintenance` | Toggle maintenance mode | Done |

### App Management (Complete)

| Command | Description | Status |
|---------|-------------|--------|
| `weg app get` | Clone/install app | Done |
| `weg app remove` | Remove app | Done |
| `weg app list` | List installed apps | Done |

### Cache & Scheduler (Complete)

| Command | Description | Status |
|---------|-------------|--------|
| `weg cache clear` | Clear Redis + pycache | Done |
| `weg scheduler status` | Check scheduler state | Done |
| `weg scheduler enable` | Enable scheduler | Done |
| `weg scheduler disable` | Disable scheduler | Done |
| `weg scheduler jobs` | List pending jobs | Done |
| `weg scheduler purge` | Purge failed jobs | Done |

### User Management (Complete)

| Command | Description | Status |
|---------|-------------|--------|
| `weg user list` | List users | Done |
| `weg user create` | Create user | Done |
| `weg user password` | Set password | Done |
| `weg user enable` | Enable user | Done |
| `weg user disable` | Disable user | Done |
| `weg user role` | Manage roles | Done |
| `weg user show` | Show user details | Done |

### Data & Fixtures (Complete)

| Command | Description | Status |
|---------|-------------|--------|
| `weg fixtures export` | Export fixtures | Done |
| `weg fixtures import` | Import fixtures | Done |
| `weg fixtures list` | List fixture files | Done |

### Database Operations (Complete)

| Command | Description | Status |
|---------|-------------|--------|
| `weg db console` | Open DB console | Done |
| `weg db backup` | Backup database | Done |
| `weg db restore` | Restore database | Done |
| `weg db trim` | Trim old data | Done |

### API Access (Complete)

| Command | Description | Status |
|---------|-------------|--------|
| `weg api get` | GET request | Done |
| `weg api post` | POST request | Done |
| `weg api put` | PUT request | Done |
| `weg api delete` | DELETE request | Done |
| `weg api call` | Call whitelisted method | Done |

### Document Operations (Complete)

| Command | Description | Status |
|---------|-------------|--------|
| `weg doc get` | Get document | Done |
| `weg doc list` | List documents | Done |
| `weg doc create` | Create document | Done |
| `weg doc delete` | Delete document | Done |
| `weg doctype list` | List doctypes | Done |
| `weg doctype show` | Show doctype schema | Done |

### Cloud & Container (Complete)

| Command | Description | Status |
|---------|-------------|--------|
| `weg cloud login` | Login to Frappe Cloud | Done |
| `weg cloud deploy` | Deploy to cloud | Done |
| `weg docker *` | Docker compose operations | Done |
| `weg image build` | Build container image | Done |

### Remote Site Development (Complete)

| Command | Description | Status |
|---------|-------------|--------|
| `weg remote clone` | Clone site customizations | Done |
| `weg remote pull` | Pull changes from remote | Done |
| `weg remote push` | Push local changes to remote | Done |
| `weg remote sync` | Bidirectional sync | Done |
| `weg remote status` | Show local vs remote diff | Done |
| `weg remote login` | Save site credentials | Done |
| `weg remote logout` | Remove site credentials | Done |
| `weg remote info` | Show remote site info | Done |

### Utilities (Complete)

| Command | Description | Status |
|---------|-------------|--------|
| `weg exec` | Run command in bench context | Done |
| `weg bench` | Run raw bench commands | Done |
| `weg doctor` | Check environment health | Done |
| `weg log` | View logs | Done |
| `weg config` | View/modify weg config | Done |
| `weg migrate` | Migrate between modes | Done |

---

## Remaining Work

### Not Planned (Low Priority)

These commands exist in bench but are rarely used in daily development:

| Bench Command | Notes |
|---------------|-------|
| `bench reinstall` | Use `site drop` + `site new` |
| `bench data-import` | Use Frappe UI or API |
| `bench destroy-all-sessions` | Use `user password --logout-all-sessions` |

### Quality Improvements

| Task | Priority | Status |
|------|----------|--------|
| Increase test coverage to 80% | Medium | In progress |
| Add integration tests | Medium | Not started |
| Complete documentation | Medium | In progress |
| Add structured error types | Low | Not started |

### Test Coverage by Package

| Package | Coverage | Status |
|---------|----------|--------|
| `internal/completion` | 84.4% | Good |
| `internal/config` | 77.5% | Good |
| `internal/runtime` | 76.8% | Good |
| `internal/state` | 76.3% | Good |
| `internal/fsutil` | 68.0% | Adequate |
| `internal/remote` | 37.8% | Adequate |
| `internal/container` | 32.5% | Adequate |
| `internal/services` | 31.5% | Needs work |
| `internal/apps` | 19.3% | Needs work |
| `internal/cloud` | 16.2% | Needs work |
| `internal/api` | 15.0% | Needs work |
| `tools` | 6.5% | Needs work |
| `cmd/*` | 0% | Biggest gap |

---

## Architecture

### File Write Safety

Config and state files use atomic writes:

```go
func atomicWrite(path string, data []byte) error {
    tmp := path + ".tmp"
    if err := os.WriteFile(tmp, data, 0644); err != nil {
        return err
    }
    return os.Rename(tmp, path)  // Atomic on POSIX
}
```

### Backup Format

```
backups/
  {site}_{datetime}.sql.gz        # Database dump
  {site}_{datetime}_files.tar.gz  # Private files (optional)
```

---

## Command Mapping: Weg vs Bench

| Weg | Bench |
|-----|-------|
| `weg new` | `bench new-app` |
| `weg init` | `bench init` |
| `weg sync` | *(declarative - no equivalent)* |
| `weg start` | `bench start` |
| `weg stop` | *(Ctrl+C)* |
| `weg build` | `bench build` |
| `weg test` | `bench run-tests` |
| `weg site new` | `bench new-site` |
| `weg site drop` | `bench drop-site` |
| `weg site use` | `bench use` |
| `weg site install` | `bench install-app` |
| `weg site backup` | `bench backup` |
| `weg site restore` | `bench restore` |
| `weg site password` | `bench set-admin-password` |
| `weg site browse` | `bench browse` |
| `weg site hosts add` | `bench add-to-hosts` |
| `weg app get` | `bench get-app` |
| `weg app remove` | `bench remove-app` |
| `weg cache clear` | `bench clear-cache` |
| `weg scheduler enable` | `bench enable-scheduler` |
| `weg scheduler disable` | `bench disable-scheduler` |
| `weg scheduler jobs` | `bench show-pending-jobs` |
| `weg scheduler purge` | `bench purge-jobs` |
| `weg user create` | `bench add-user` |
| `weg user disable` | `bench disable-user` |
| `weg fixtures export` | `bench export-fixtures` |
| `weg exec` | `bench execute` |
| `weg update` | `bench update` |
| `weg version` | `bench version` |
| `weg api *` | *(new - direct API access)* |
| `weg cloud *` | *(new - Frappe Cloud integration)* |
| `weg docker *` | *(new - Docker operations)* |
| `weg image *` | *(new - container images)* |
| `weg doc *` | *(new - document operations)* |
| `weg doctype *` | *(new - doctype operations)* |
| `weg remote *` | *(new - remote site development)* |

---

## Open Questions

1. **Windows support?** - devbox works on WSL, native Windows unclear
2. **Offline mode?** - Cache dependencies for air-gapped development?
3. **Plugin system?** - Allow custom commands via hooks?
