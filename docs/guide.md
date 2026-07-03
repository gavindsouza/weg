# weg Guide

A practical tour of weg beyond the [README](../README.md) quickstart. Everything here works against the current release; each command also has detailed `--help` output with more examples.

## Choosing a starting command

Four commands create or adopt projects — pick by situation:

| Situation | Command |
|-----------|---------|
| Brand-new Frappe app | `weg new myapp` |
| Brand-new multi-app bench | `weg create mybench` |
| Existing app or bench checkout | `weg init` |
| Just want to try an app | `weg run frappe/hrms` |

`weg init` is context-aware: in a fresh directory it runs interactive setup, in a Frappe app it adds `[tool.weg]` to `pyproject.toml`, and in a traditional bench it generates `weg.toml` from the existing structure. Force a style with `--app` or `--bench`.

`weg run` builds a disposable environment in a temp directory (keep it with `--keep`, place it with `--dir`), resolves the app's dependencies, creates a site, and starts the server.

## Creating a new app

```bash
weg new my-awesome-app                 # New directory
weg new .                              # In the current directory
weg new my-app --version 15 --database mariadb
weg new my-app -y                      # Non-interactive, use defaults
```

This creates the app module (`hooks.py`, `__init__.py`), a `pyproject.toml` with a `[tool.weg]` section, and a hidden `.weg/` development environment (skip with `--skip-init`).

## Project structure

### App-centric

```
my-frappe-app/
├── my_frappe_app/
│   ├── __init__.py
│   ├── hooks.py
│   └── ...
├── pyproject.toml      # With [tool.weg] section
└── .weg/               # Hidden bench (auto-created)
    ├── apps/
    ├── sites/
    └── state.json
```

### Bench-centric

```
my-bench/
├── weg.toml
├── apps/
│   ├── frappe/
│   └── erpnext/
├── sites/
│   └── mysite.localhost/
└── .weg/
    └── state.json
```

Switch between the two layouts any time with `weg convert app` / `weg convert bench`.

Weg works from any subdirectory of a project, and from anywhere with `-C`:

```bash
weg -C ~/projects/myapp status
```

## Configuration reference

### weg.toml (bench-centric)

```toml
[bench]
name = "my-project"

[frappe]
version = "15"                 # "14", "15", "16", or a branch like "version-15"
database = "mariadb"           # mariadb, postgres, sqlite

[apps.erpnext]
url = "https://github.com/frappe/erpnext"
branch = "version-15"

[apps.myapp]
path = "../myapp"              # Local app for development
excluded = false               # Set true to skip during sync

[[sites]]
name = "mysite.localhost"
default = true
apps = ["frappe", "erpnext"]

[services.web]
port = 8000

[services.workers]             # Queue name -> instance count
short = 1
default = 2
long = 1
```

### pyproject.toml [tool.weg] (app-centric)

```toml
[tool.weg.compatibility]
frappe = ["14", "15", "16"]    # Versions this app supports
databases = ["mariadb", "postgres"]

[tool.weg.dev]
frappe = "15"                  # Version to develop against
database = "mariadb"

[[tool.weg.dependencies.apps]]
name = "erpnext"
url = "https://github.com/frappe/erpnext"
branch = "version-15"
```

`weg test --all-versions` uses the `compatibility.frappe` list to run your tests against every supported version.

### App-defined services

Apps can declare extra devbox packages and processes they need. When you `weg sync`, packages are added to the devbox environment and processes are merged into the generated `process-compose.yaml`:

```toml
[tool.weg.services]
packages = ["imagemagick@latest"]

[tool.weg.services.processes.my-worker]
command = "python -m myapp.worker"
environment = { QUEUE = "priority" }
depends_on = ["web"]
```

This makes apps self-contained: anyone cloning the app gets its service dependencies automatically.

### Customizing generated services

Weg generates `process-compose.yaml` for development services and automatically includes an optional `process-compose.override.yaml` if present:

```yaml
# process-compose.override.yaml
processes:
  web:
    environment:
      - GUNICORN_WORKERS=4
  watch:
    disabled: true            # Disable a generated process
  mailhog:
    command: mailhog          # Add your own process
```

## The sync workflow

Weg's core loop: edit config, then apply it.

```bash
weg add frappe/erpnext version-15   # Edit config only
weg remove erpnext                  # Edit config only
weg sync --dry-run                  # Preview what would change
weg sync                            # Apply (interactive confirmation)
weg sync -y                         # Apply without confirmation
weg status                          # Am I in sync?
```

Sync installs new apps, removes deleted ones, updates changed branches, and creates or removes sites to match the configuration.

To act immediately instead of declaratively, use the `weg app` commands:

```bash
weg app get frappe/hrms             # Clone + install now, with dependency resolution
weg app get frappe/hrms --skip-deps
weg app switch frappe version-15    # Change branch and reinstall deps
weg app exclude erpnext             # Temporarily skip during sync
weg app include erpnext
weg app reinstall myapp
```

## Updating and upgrading

Two distinct operations:

```bash
weg update                  # Update all apps within the current Frappe version
weg update frappe           # Update one app
weg update --pull           # Only pull code, skip dependency install
weg update --no-build       # Skip asset rebuild

weg upgrade                 # Move to the NEXT major version (14 → 15 → 16 → develop)
weg upgrade --dry-run       # Show current → next without changing anything
weg upgrade --no-migrate    # Skip database migrations
```

`weg upgrade` handles the full process: updates configuration, regenerates the devbox environment (Python/Node versions), checks out the new frappe branch, reinstalls Python and Node dependencies, and runs database migrations.

To update weg itself: `weg self update`.

## Sites

```bash
weg site new mysite.localhost --install-app erpnext --set-default
weg site use mysite.localhost
weg site install mysite.localhost erpnext     # App must be in the bench already
weg site browse                               # Open in browser as Administrator
weg site browse --user hr@test.com

weg site backup --with-files                  # → .weg/backups/{site}_{datetime}.sql.gz
weg site restore backup.sql.gz                # Restore default site
weg site restore db.sql.gz --files files.tar.gz
weg site password                             # Reset Administrator password (prompts)
weg site password --logout-all-sessions

weg site maintenance on                       # Only Administrator can log in
weg site hosts add                            # Add sites to /etc/hosts (needs sudo)
weg site config get                           # site_config.json access
weg site config set custom_key value
```

## Direct data access

### Documents and APIs

```bash
weg api get User                              # List documents
weg api get User/Administrator                # Get one document
weg api get User -F '{"enabled":1}' --fields '["name","email"]' --limit 10
weg api post ToDo -d '{"description":"My task","priority":"High"}'
weg api put User/test@example.com -d '{"first_name":"Updated"}'
weg api delete User/test@example.com
weg api call frappe.client.get_count doctype=User
weg api run "Sales Invoice/INV-001" submit    # Call a document method
```

Local mode runs directly via Python as Administrator (no HTTP). Remote mode targets any site over HTTP:

```bash
weg api -U https://site.frappe.cloud -k KEY -K SECRET get User
```

Credentials resolve from flags, then `WEG_API_KEY`/`WEG_API_SECRET`, then `~/.config/weg/credentials.toml` (see `weg remote login`).

Higher-level document operations:

```bash
weg doc get User Administrator
weg doc list User --limit 10
weg doc export Role "System Manager"          # To JSON
weg doc import fixtures/role.json
weg doc rename Customer "Old Name" "New Name"
weg doc field get User Administrator email    # Single-field fast path
weg doc field set User Administrator enabled 0
```

### Database

```bash
weg db migrate                                # Run migrations (alias: weg migrate)
weg db console                                # Interactive MariaDB/Postgres shell
weg db query "SELECT name, email FROM tabUser LIMIT 5"
echo "SELECT 1" | weg db query -              # From stdin
weg db trim --days 30                         # Trim log tables
```

### Python

`weg py` runs Python with `frappe` already imported and connected — no PYTHONPATH, no `frappe.init()` boilerplate:

```bash
weg py "print(frappe.get_all('User', pluck='name'))"
weg py script.py
echo "print(frappe.db.count('User'))" | weg py -
weg py --site dev.localhost "print(frappe.local.site)"
```

## Running commands in the bench context

For anything weg doesn't wrap, drop into the bench environment:

```bash
weg exec -- bench migrate                     # Any command, correct env + cwd
weg exec --site mysite.localhost -- python -c "import frappe"
weg bench migrate                             # bench passthrough shortcut
weg bench frappe --help                       # All raw frappe/bench commands
```

## Remote-site development

Develop against a hosted Frappe site (Frappe Cloud or self-hosted) with no local bench:

```bash
weg remote login https://mysite.frappe.cloud  # Save API creds (~/.config/weg, 0600)
weg remote clone https://mysite.frappe.cloud mysite
cd mysite
```

The clone mirrors the site's customizations as files — Custom DocTypes, Custom Fields, Property Setters, Client/Server Scripts, Reports, Print Formats, Workflows, Notifications, Letter Heads — and reconstructs each document's version history into git commits. History fetching streams to an on-disk cache and is resumable: re-run an interrupted clone and it picks up where it stopped. Useful flags:

```bash
weg remote clone <url> --no-history           # Fast single-commit clone
weg remote clone <url> --modules=Custom,Selling
weg remote clone <url> --exclude=workflow
```

Day-to-day flow:

```bash
weg remote status                             # Local vs remote diff
weg remote pull -m "Sync remote changes"
weg remote push -n                            # Dry-run preview
weg remote push -u                            # Include uncommitted changes
weg remote sync -m "Add priority field"       # Pull, commit, push in one step
weg remote info
```

### Workspace: edit scripts as real source files

Server/Client Scripts live inside JSON documents, which editors can't help with. The workspace extracts them into typed files:

```bash
weg workspace expand          # JSON → weg_workspace/*.py, *.js, *.sql, *.html
# ...edit with full IDE support...
weg workspace collapse        # Pack changes back into the JSON
weg workspace status          # What's out of sync
weg workspace watch           # Auto-collapse on save
weg workspace init            # Set up with pre-commit hooks
```

## Logs

```bash
weg log tail                  # Tail all logs
weg log tail web              # web, worker, schedule, error, all
weg log tail --lines 100
weg log show                  # Recent entries
weg log list
weg log clear
```

## Shell completions

```bash
# Bash — add to ~/.bashrc
eval "$(weg completion bash)"

# Zsh — add to ~/.zshrc
eval "$(weg completion zsh)"

# Fish
weg completion fish > ~/.config/fish/completions/weg.fish
```

For faster shell startup, write the output to your completions directory instead of using `eval`.

## Troubleshooting

**Start with the doctors:**

```bash
weg doctor          # Project health: devbox, services, site config, symlinks, Python
weg self doctor     # System compatibility: OS, required tools
```

`weg doctor` exits non-zero when a check fails, so it works as a CI gate.

**"devbox not found" / "uv not found"** — install the required tooling:

```bash
weg self install-tools
```

**"not a weg-managed project"** — run `weg init` in the project directory first, or use `weg new` / `weg create` to start fresh.

**Running from outside the project** — use `-C`:

```bash
weg -C /path/to/project status
```

**Debugging weg itself** — crank up verbosity:

```bash
weg -vv sync                          # Debug level
weg --log-level trace --debug-categories net,git remote pull
```

See [CLI_CONVENTIONS.md](CLI_CONVENTIONS.md) for the full verbosity, output-format, and exit-code contract.
