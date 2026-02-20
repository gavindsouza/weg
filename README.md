# Weg

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License: Apache 2.0](https://img.shields.io/github/license/gavindsouza/weg)](LICENSE)

The fast way to develop Frappe apps.

Weg means "way" in German and "speed" in Marathi/Sanskrit — a modern replacement for Frappe's `bench` CLI with declarative configuration and faster tooling.

## Three Development Modes

Weg supports three distinct workflows for Frappe development:

### 1. App-Centric Development

Your app is the project root. The bench infrastructure is hidden in `.weg/`. Ideal for developing a single Frappe app with modern tooling.

```bash
weg new myapp
cd myapp
weg start
```

Configuration lives in `pyproject.toml [tool.weg]`.

### 2. Bench-Centric Development

Traditional bench directory structure. Use this when working with multiple apps or migrating from existing bench setups.

```bash
cd /path/to/frappe-bench
weg init
weg start
```

Configuration lives in `weg.toml`.

### 3. Remote-Site Development

Work with remote Frappe sites (like Frappe Cloud) without direct bench access. Clone customizations locally, edit with any tools, and sync changes back.

```bash
weg remote clone https://mysite.frappe.cloud mysite
cd mysite
# Edit Client Scripts, Server Scripts, Custom Fields locally
weg remote push -m "Add priority field to Todo"
```

This creates a git-backed directory mirroring the site's customizations, enabling version control, team collaboration, and AI-assisted editing.

## Key Features

- **Declarative configuration** via `weg.toml` or `pyproject.toml`
- **Modern tooling** - devbox (Nix), uv (fast Python), process-compose
- **Direct API access** - `weg api` without HTTP overhead
- **Container support** - Docker Compose generation and production image builds
- **MCP server** - AI assistant integration via Model Context Protocol
- **70+ commands** covering all common Frappe development workflows
- **Works from anywhere** - run commands from any subdirectory within your project

## Installation

```bash
# Download the latest binary
curl -fsSL https://github.com/gavindsouza/weg/releases/latest/download/weg-$(uname -s)-$(uname -m) -o weg
chmod +x weg
mkdir -p ~/.local/bin
mv weg ~/.local/bin/

# Add to PATH if not already (add to ~/.bashrc or ~/.zshrc)
export PATH="$HOME/.local/bin:$PATH"

# Or build from source
git clone https://github.com/gavindsouza/weg
cd weg
go build -o weg .
mv weg ~/.local/bin/
```

## Quick Start

### App-Centric (new app)

```bash
weg new myapp
cd myapp
weg start
weg site browse    # Open in browser (auto-login as Administrator)
```

### Bench-Centric (existing bench)

```bash
cd /path/to/frappe-bench
weg init
weg start
```

### Remote-Site (Frappe Cloud or any remote site)

```bash
weg remote clone https://mysite.frappe.cloud mysite
cd mysite
# Edit customizations locally...
weg remote status  # See what changed
weg remote push    # Push changes to remote
```

## Common Commands

### Site Management

```bash
weg site list                    # List all sites
weg site new mysite.localhost    # Create new site
weg site drop mysite.localhost   # Delete site
weg site use mysite.localhost    # Set default site
weg site backup                  # Backup current site
weg site restore backup.sql.gz   # Restore from backup
weg site password                # Reset admin password
weg site browse                  # Open site in browser
```

### App Management

```bash
weg app list                     # List installed apps
weg app get erpnext              # Install ERPNext
weg app get https://github.com/user/custom-app
weg app remove custom-app        # Remove an app
weg site install custom-app      # Install app on site
```

### Development

```bash
weg start                        # Start all services
weg stop                         # Stop all services
weg build                        # Build frontend assets
weg build --app myapp            # Build specific app
weg test                         # Run tests
weg test --module myapp.tests    # Run specific tests
weg console                      # Open Python console
weg db console                   # Open database console
```

### Cache & Maintenance

```bash
weg cache clear                  # Clear Redis + pycache
weg scheduler status             # Check scheduler status
weg scheduler enable             # Enable background jobs
weg scheduler disable            # Disable background jobs
weg scheduler jobs               # List pending jobs
```

### API Access

```bash
# Direct API calls without HTTP overhead
weg api call frappe.client.get_count doctype=User
weg doc get User Administrator   # Get a document
weg doc list User --limit 10     # List documents
weg doctype list                 # List all doctypes
```

### Remote Site Development

```bash
weg remote clone <url> <dir>     # Clone site customizations
weg remote pull                  # Pull changes from remote
weg remote push                  # Push local changes to remote
weg remote push -m "message"     # Push with commit message
weg remote status                # Show local vs remote diff
weg remote sync                  # Bidirectional sync
weg remote login <url>           # Save credentials for a site
```

### Frappe Cloud

```bash
weg cloud login                  # Authenticate with Frappe Cloud
weg cloud sites                  # List your sites
weg cloud deploy mysite          # Deploy to cloud
weg cloud logs mysite            # View site logs
```

### Docker & Containers

```bash
# Docker Compose for local development
weg docker init                  # Generate docker-compose.yml
weg docker init --mode prod      # Generate for production
weg docker up                    # Start containers
weg docker down                  # Stop containers
weg docker logs                  # View container logs
weg docker ps                    # List running containers

# Build production images
weg image build                  # Build container image
weg image build --tag myapp:v1   # Custom tag
weg image build --push           # Build and push to registry
weg image list                   # List local images
```

## Configuration

Weg uses `weg.toml` for bench-centric projects or `pyproject.toml [tool.weg]` for app-centric projects.

### weg.toml (bench-centric)

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
default = 2
long = 1
```

### pyproject.toml (app-centric)

```toml
[tool.weg]
frappe_version = "15"

[tool.weg.dependencies]
erpnext = { url = "https://github.com/frappe/erpnext", branch = "version-15" }

[tool.weg.sites]
default = "dev.localhost"
```

## Customizing Services

### Process Compose (Development)

Weg generates `process-compose.yaml` for running development services. You can customize it by creating `process-compose.override.yaml`:

```yaml
# process-compose.override.yaml
processes:
  web:
    environment:
      - GUNICORN_WORKERS=4

  # Disable a process
  watch:
    disabled: true

  # Add a custom process
  mailhog:
    command: mailhog
    readiness_probe:
      http_get:
        port: 8025
```

The override file is automatically included when present (uses process-compose's native include feature).

### Docker Compose (Containerized)

For containerized development or production, use `weg docker init` to generate a `docker-compose.yml` with all required services (web, workers, scheduler, socketio, database, Redis).

Options include `--mode prod` for production settings, `--no-db` for external databases, and `--web-port` to customize ports. Edit the generated file directly or use Docker Compose overrides for further customization.

### Container Images

Use `weg image build` to create OCI-compliant production images. The multi-stage Dockerfile generates specialized targets (web, worker, scheduler, socketio) optimized for deployment to Docker, Podman, or Kubernetes.

## Shell Completions

The easiest way to enable completions:

```bash
# Bash - add to ~/.bashrc
eval "$(weg completion bash)"

# Zsh - add to ~/.zshrc
eval "$(weg completion zsh)"

# Fish - run once
weg completion fish | source
# Or persist: weg completion fish > ~/.config/fish/completions/weg.fish
```

For faster shell startup (optional), you can cache the completions:

```bash
# Bash
weg completion bash > ~/.local/share/bash-completion/completions/weg

# Zsh (if using Oh My Zsh)
weg completion zsh > ~/.oh-my-zsh/completions/_weg
```

## vs bench

| Feature | weg | bench |
|---------|-----|-------|
| Configuration | Declarative (TOML) | Imperative (commands) |
| Development modes | App-centric, bench-centric, remote-site | Bench-centric only |
| Python management | uv (fast) | pip |
| System dependencies | devbox/Nix (reproducible) | Manual |
| Process management | process-compose | honcho/supervisord |
| Container support | Built-in (Docker Compose + image builds) | frappe_docker (separate) |
| API access | Direct (no HTTP) | Via HTTP |
| Remote site editing | Built-in (git-backed) | Not available |
| Cloud integration | Built-in | Separate tool |

## AI Integration

Weg includes an MCP (Model Context Protocol) server that lets AI assistants manage your Frappe environment:

```bash
weg mcp install   # Configure for Claude Code / other MCP clients
```

The MCP server exposes 12 tools for running Python, calling APIs, managing sites, and more — replacing common anti-patterns like manual `bench` activation or direct `frappe` imports.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for setup, testing, and code style guidelines.

## Requirements

- Go 1.24+ (for building from source)
- [devbox](https://www.jetify.com/devbox) (installed automatically on first use)
- Git

## License

MIT
