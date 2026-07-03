# weg

[![CI](https://github.com/gavindsouza/weg/actions/workflows/ci.yml/badge.svg)](https://github.com/gavindsouza/weg/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/gavindsouza/weg?include_prereleases)](https://github.com/gavindsouza/weg/releases/latest)
[![Go Report Card](https://goreportcard.com/badge/github.com/gavindsouza/weg)](https://goreportcard.com/report/github.com/gavindsouza/weg)
[![License: Apache 2.0](https://img.shields.io/github/license/gavindsouza/weg)](LICENSE)

**The fast way to develop Frappe apps.** *Weg* means "way" in German and "speed" (वेग) in Marathi/Sanskrit — a modern replacement for Frappe's `bench` CLI.

<!-- TODO: VHS demo of weg new → weg start → weg site browse -->

## Why weg

- **One TOML file, whole environment.** Declare apps, sites, and services in `weg.toml` or `pyproject.toml [tool.weg]`; `weg sync` makes reality match. No more replaying `bench` incantations from memory.
- **Fast by construction.** A single static Go binary orchestrating [devbox](https://www.jetify.com/devbox) (reproducible system deps via Nix), [uv](https://github.com/astral-sh/uv) (fast Python), and [process-compose](https://github.com/F1bonacc1/process-compose) (service management).
- **Your shell is a Frappe client.** `weg api get User`, `weg doc field set`, `weg db query "SELECT ..."`, `weg py "print(frappe.get_all('ToDo'))"` — direct document, API, and database access without HTTP boilerplate or console copy-paste.
- **Remote sites with real git history.** `weg remote clone` pulls a site's customizations (Server Scripts, Client Scripts, Custom Fields, …) into a local git repo — and reconstructs the full document version history into commits, streamed and resumable. Then edit locally and `weg remote push`.
- **Built for AI workflows.** `weg mcp install` gives Claude Code and other MCP clients 12 structured tools for driving your Frappe environment.
- **Scriptable everywhere.** `-o json` on status/list/introspection commands, meaningful exit codes, and `weg doctor` that fails loudly enough to gate CI.

## Install

**Download a release binary** (Linux/macOS):

```bash
curl -fsSL https://github.com/gavindsouza/weg/releases/latest/download/weg-$(uname -s)-$(uname -m) -o weg
chmod +x weg && mkdir -p ~/.local/bin && mv weg ~/.local/bin/
```

**Or with Go:**

```bash
go install github.com/gavindsouza/weg@latest
```

**Or build from source** (Go 1.24+):

```bash
git clone https://github.com/gavindsouza/weg && cd weg
go build -o ~/.local/bin/weg .
```

Weg needs `git`; everything else (devbox, direnv, …) can be installed with `weg self install-tools`. Run `weg self doctor` to check your system.

## Quickstart

Weg supports three development modes. Pick the one that matches how you work.

### 1. App-centric — your app is the project root

The bench lives hidden in `.weg/`; config lives in `pyproject.toml [tool.weg]`. Ideal for developing a single Frappe app with modern tooling.

```bash
weg new myapp
cd myapp
weg start
weg site browse    # Opens the site, auto-logged-in as Administrator
```

### 2. Bench-centric — traditional bench layout

`apps/` and `sites/` at the root; config lives in `weg.toml`. Use this for multi-app projects or when adopting an existing bench.

```bash
cd frappe-bench
weg init
weg start
```

### 3. Remote-site — no bench at all

Work on a hosted site (Frappe Cloud or any Frappe site) by cloning its customizations into a local git repo:

```bash
weg remote clone https://mysite.frappe.cloud mysite
cd mysite
# Edit Client Scripts, Server Scripts, Custom Fields... as local files
weg remote push -n                     # Preview what would change
weg remote sync -m "Add priority field to Todo"
```

The clone reconstructs each document's version history into git commits, so `git log` and `git blame` work on customizations that never had version control. History fetching is streamed and resumable — interrupt a large clone and run it again to pick up where it left off (or pass `--no-history` for a fast single-commit clone).

Just want to try an app? `weg run https://github.com/frappe/hrms` clones it, builds a throwaway environment, creates a site, and starts the server.

## Command tour

Run `weg --help` for the full grouped listing; every command has detailed `--help` with examples. Highlights:

### Getting started

```bash
weg new myapp                # Create a new Frappe app (app-centric)
weg create mybench           # Create a new bench (bench-centric)
weg init                     # Adopt an existing app or bench directory
weg run frappe/hrms          # Disposable environment for any app
weg scaffold ai              # Add CLAUDE.md + AI agent skills to a project
```

### Daily development

```bash
weg start                    # Start db, redis, web, workers, scheduler, watcher
weg start -f                 # Same, in the foreground with a TUI
weg stop                     # Stop everything
weg status                   # What's installed, what's running, is sync needed
weg doctor                   # Health checks; exits non-zero on failure
weg build                    # Build frontend assets (weg build watch for watch mode)
weg test                     # Run app tests (--all-versions for a version matrix)
weg log tail web             # Tail web/worker/schedule/error logs
```

### Site & data

```bash
weg site new mysite.localhost        # Create a site
weg site use mysite.localhost        # Set the default site
weg site backup --with-files         # Backup to .weg/backups/
weg site maintenance on              # Maintenance mode
weg site hosts add                   # Add sites to /etc/hosts

weg api get User -F '{"enabled":1}'  # REST-style document access, no HTTP setup
weg api call frappe.ping             # Call any whitelisted method
weg doc field set User Administrator enabled 0
weg db migrate                       # Run database migrations
weg db query "SELECT name FROM tabUser LIMIT 5"
weg py "print(frappe.db.count('User'))"   # Python with frappe pre-connected
weg exec -- bench migrate            # Escape hatch: any command in the bench env
```

### Apps

```bash
weg add frappe/erpnext version-15    # Declare an app in config...
weg sync                             # ...and apply the config
weg app get frappe/hrms              # Or clone + install immediately (with deps)
weg app switch frappe version-15     # Switch an app's branch
weg update                           # Update all apps within the current version
weg upgrade --dry-run                # Preview a major-version upgrade (15 → 16)
```

### Deployment

```bash
weg docker init                      # Generate docker-compose.yml (--mode prod)
weg image build --target web         # Multi-stage OCI images for production
weg cloud login                      # Frappe Cloud: deploy, logs, marketplace
weg cloud deploy mysite.frappe.cloud
```

### Remote sites

```bash
weg remote login https://mysite.frappe.cloud   # Save credentials (0600, global)
weg remote clone https://mysite.frappe.cloud   # Clone with full version history
weg remote status                              # Local vs remote diff
weg remote sync -m "description"               # Pull, commit, push

weg workspace expand                 # Extract scripts from JSON into .py/.js files
weg workspace collapse               # Pack IDE edits back into JSON
```

See [docs/guide.md](docs/guide.md) for the full guide.

## Configuration

### `weg.toml` (bench-centric)

```toml
[frappe]
version = "15"
database = "mariadb"

[apps.erpnext]
url = "https://github.com/frappe/erpnext"
branch = "version-15"

[[sites]]
name = "mysite.localhost"
default = true
apps = ["frappe", "erpnext"]

[services.workers]
short = 1
long = 1
```

### `pyproject.toml [tool.weg]` (app-centric)

```toml
[tool.weg.compatibility]
frappe = ["15", "16"]          # Versions your app supports
databases = ["mariadb"]

[tool.weg.dev]
frappe = "15"                  # Version to develop against
database = "mariadb"

[[tool.weg.dependencies.apps]]
name = "erpnext"
url = "https://github.com/frappe/erpnext"
branch = "version-15"
```

Apps can also declare extra devbox packages and processes they need under `[tool.weg.services]` — see the [guide](docs/guide.md#app-defined-services) for details, including customizing generated services with `process-compose.override.yaml`.

## weg vs bench

| Feature | weg | bench |
|---------|-----|-------|
| Configuration | Declarative (TOML) | Imperative (commands) |
| Development modes | App-centric, bench-centric, remote-site | Bench-centric only |
| Python management | uv (fast) | pip |
| System dependencies | devbox/Nix (reproducible) | Manual |
| Process management | process-compose | honcho/supervisord |
| Container support | Built-in (Compose + image builds) | frappe_docker (separate) |
| API access | Direct (no HTTP) | Via HTTP |
| Remote site editing | Built-in (git-backed, with history) | Not available |
| Cloud integration | Built-in | Separate tool |

<details>
<summary><strong>Command mapping: coming from bench?</strong></summary>

| weg | bench |
|-----|-------|
| `weg new` | `bench new-app` |
| `weg init` | `bench init` |
| `weg sync` | *(declarative — no equivalent)* |
| `weg start` / `weg stop` | `bench start` / *(Ctrl+C)* |
| `weg build` | `bench build` |
| `weg test` | `bench run-tests` |
| `weg db migrate` | `bench migrate` |
| `weg db console` | `bench mariadb` |
| `weg site new` | `bench new-site` |
| `weg site drop` | `bench drop-site` |
| `weg site use` | `bench use` |
| `weg site install` | `bench install-app` |
| `weg site backup` / `weg site restore` | `bench backup` / `bench restore` |
| `weg site password` | `bench set-admin-password` |
| `weg site browse` | `bench browse` |
| `weg site hosts add` | `bench add-to-hosts` |
| `weg app get` | `bench get-app` |
| `weg app remove` | `bench remove-app` |
| `weg cache clear` | `bench clear-cache` |
| `weg scheduler enable` / `disable` | `bench enable-scheduler` / `disable-scheduler` |
| `weg scheduler jobs` | `bench show-pending-jobs` |
| `weg scheduler purge` | `bench purge-jobs` |
| `weg user create` | `bench add-user` |
| `weg user disable` | `bench disable-user` |
| `weg fixtures export` | `bench export-fixtures` |
| `weg api call` | `bench execute` |
| `weg update` | `bench update` |
| `weg version` | `bench version` |
| `weg convert` | *(new — switch app-centric ⇄ bench-centric layout)* |
| `weg exec -- <cmd>` | *(new — run any command in the bench env)* |
| `weg api` / `weg doc` / `weg doctype` / `weg py` | *(new — direct data access)* |
| `weg remote` / `weg workspace` | *(new — remote-site development)* |
| `weg cloud` / `weg docker` / `weg image` | *(new — deployment)* |
| `weg mcp` | *(new — AI integration)* |

Anything not covered still works via passthrough: `weg bench <any-bench-command>`.

</details>

## AI integration

```bash
weg mcp install   # Adds weg to .mcp.json (merges with existing servers)
```

Weg ships an MCP (Model Context Protocol) server exposing 12 structured tools — running Python against your site, calling APIs, managing sites and services — so AI assistants like Claude Code drive your environment through safe, typed operations instead of guessing at shell commands.

## Global flags & scripting

```bash
weg -C ~/projects/myapp status    # Run against another directory (like git -C)
weg site list -o json | jq .      # JSON output on status/version/doctor/config
                                  # show/site list/app list/cloud marketplace
weg -v sync                       # Verbosity: -v, -vv, -vvv (or --log-level)
weg -y site drop old.localhost    # Assume yes for prompts
weg doctor || exit 1              # Non-zero exit on failed checks — CI-friendly
```

Exit codes are meaningful (0 success, 2 usage, 3 config, 5 network, 6 not found, …) — the full contract lives in [docs/CLI_CONVENTIONS.md](docs/CLI_CONVENTIONS.md).

## Status & license

Weg is pre-1.0 software: used daily for real Frappe development, but interfaces may still change between releases. Design decisions are recorded in [docs/decisions/](docs/decisions/).

Licensed under [Apache 2.0](LICENSE).
