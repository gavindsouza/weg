# Weg Product Roadmap

## Executive Summary

Weg is a modern CLI replacement for Frappe's `bench` tool, offering declarative configuration, faster tooling (devbox, uv, process-compose), and an app-centric development model.

### Current State

| Metric | Value | Target | Status |
|--------|-------|--------|--------|
| Commands implemented | 70+ | ~70 | ✅ Complete |
| Test coverage | ~40% | 80%+ | 🔄 In progress |
| Critical missing features | 0 | 0 | ✅ Complete |
| Known TODOs/bugs | 0 | 0 | ✅ Complete |
| Documentation completeness | 60% | 100% | 🔄 In progress |

### Key Differentiators from Bench

1. **Declarative config** - `weg.toml` / `pyproject.toml [tool.weg]` vs imperative commands
2. **App-centric mode** - Your app is the project root, bench hidden in `.weg/`
3. **Modern tooling** - devbox (Nix), uv (fast Python), process-compose
4. **Direct API access** - `weg api` without HTTP overhead
5. **Multi-version testing** - `weg test --versions 14,15,16`
6. **Cloud-native** - Built-in Frappe Cloud, Docker, container image support

---

## Feature Status

### Core Commands (All Complete ✅)

| Command | Description | Status |
|---------|-------------|--------|
| `weg new` | Create new Frappe app | ✅ |
| `weg init` | Initialize weg in directory | ✅ |
| `weg sync` | Apply configuration changes | ✅ |
| `weg start` | Start development servers | ✅ |
| `weg stop` | Stop development services | ✅ |
| `weg build` | Build assets and frontend | ✅ |
| `weg test` | Run tests | ✅ |
| `weg update` | Update apps to latest | ✅ |
| `weg version` | Show version info | ✅ |

### Site Management (All Complete ✅)

| Command | Description | Status |
|---------|-------------|--------|
| `weg site new` | Create new site | ✅ |
| `weg site drop` | Delete site | ✅ |
| `weg site list` | List all sites | ✅ |
| `weg site use` | Set default site | ✅ |
| `weg site install` | Install app on site | ✅ |
| `weg site backup` | Backup database + files | ✅ |
| `weg site restore` | Restore from backup | ✅ |
| `weg site password` | Set/reset user password | ✅ |
| `weg site browse` | Open site in browser | ✅ |
| `weg site config` | Manage site config | ✅ |
| `weg site hosts` | Manage /etc/hosts entries | ✅ |
| `weg site maintenance` | Toggle maintenance mode | ✅ |

### App Management (All Complete ✅)

| Command | Description | Status |
|---------|-------------|--------|
| `weg app get` | Clone/install app | ✅ |
| `weg app remove` | Remove app | ✅ |
| `weg app list` | List installed apps | ✅ |

### Cache & Scheduler (All Complete ✅)

| Command | Description | Status |
|---------|-------------|--------|
| `weg cache clear` | Clear Redis + pycache | ✅ |
| `weg scheduler status` | Check scheduler state | ✅ |
| `weg scheduler enable` | Enable scheduler | ✅ |
| `weg scheduler disable` | Disable scheduler | ✅ |
| `weg scheduler jobs` | List pending jobs | ✅ |
| `weg scheduler purge` | Purge failed jobs | ✅ |

### User Management (All Complete ✅)

| Command | Description | Status |
|---------|-------------|--------|
| `weg user list` | List users | ✅ |
| `weg user create` | Create user | ✅ |
| `weg user password` | Set password | ✅ |
| `weg user enable` | Enable user | ✅ |
| `weg user disable` | Disable user | ✅ |
| `weg user role` | Manage roles | ✅ |
| `weg user show` | Show user details | ✅ |

### Data & Fixtures (All Complete ✅)

| Command | Description | Status |
|---------|-------------|--------|
| `weg fixtures export` | Export fixtures | ✅ |
| `weg fixtures import` | Import fixtures | ✅ |
| `weg fixtures list` | List fixture files | ✅ |

### Database Operations (All Complete ✅)

| Command | Description | Status |
|---------|-------------|--------|
| `weg db console` | Open DB console | ✅ |
| `weg db backup` | Backup database | ✅ |
| `weg db restore` | Restore database | ✅ |
| `weg db trim` | Trim old data | ✅ |

### API Access (All Complete ✅)

| Command | Description | Status |
|---------|-------------|--------|
| `weg api get` | GET request | ✅ |
| `weg api post` | POST request | ✅ |
| `weg api put` | PUT request | ✅ |
| `weg api delete` | DELETE request | ✅ |
| `weg api call` | Call whitelisted method | ✅ |

### Document Operations (All Complete ✅)

| Command | Description | Status |
|---------|-------------|--------|
| `weg doc get` | Get document | ✅ |
| `weg doc list` | List documents | ✅ |
| `weg doc create` | Create document | ✅ |
| `weg doc delete` | Delete document | ✅ |
| `weg doctype list` | List doctypes | ✅ |
| `weg doctype show` | Show doctype schema | ✅ |

### Cloud & Container (All Complete ✅)

| Command | Description | Status |
|---------|-------------|--------|
| `weg cloud login` | Login to Frappe Cloud | ✅ |
| `weg cloud deploy` | Deploy to cloud | ✅ |
| `weg docker *` | Docker compose operations | ✅ |
| `weg image build` | Build container image | ✅ |

### Utilities (All Complete ✅)

| Command | Description | Status |
|---------|-------------|--------|
| `weg exec` | Run command in bench context | ✅ |
| `weg bench` | Run raw bench commands | ✅ |
| `weg doctor` | Check environment health | ✅ |
| `weg log` | View logs | ✅ |
| `weg config` | View/modify weg config | ✅ |
| `weg migrate` | Migrate between modes | ✅ |

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
| Increase test coverage to 80% | Medium | 🔄 In progress |
| Add integration tests | Medium | Not started |
| Complete documentation | Medium | 🔄 In progress |
| Add structured error types | Low | Not started |

### Test Coverage by Package

| Package | Coverage | Status |
|---------|----------|--------|
| `internal/state` | 77.5% | ✅ Good |
| `internal/completion` | 84.4% | ✅ Good |
| `internal/config` | ~60% | 🔄 Adequate |
| `internal/api` | 15% | ⚠️ Needs work |
| `internal/services` | ~10% | ⚠️ Needs work |
| `internal/apps` | 0% | ⚠️ Needs work |
| `internal/cloud` | 0% | ⚠️ Needs work |

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

---

## Open Questions

1. **Windows support?** - devbox works on WSL, native Windows unclear
2. **Offline mode?** - Cache dependencies for air-gapped development?
3. **Plugin system?** - Allow custom commands via hooks?
