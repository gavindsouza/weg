# Weg

A modern CLI replacement for Frappe's `bench` tool with declarative configuration and faster tooling.

## What is Weg?

Weg (वेग - Marathi/Sanskrit for "speed") is a command-line tool for managing Frappe development environments. It provides:

- **Declarative configuration** via `weg.toml` or `pyproject.toml`
- **App-centric development** - your app is the project root, bench hidden in `.weg/`
- **Modern tooling** - devbox (Nix), uv (fast Python), process-compose
- **Direct API access** - `weg api` without HTTP overhead
- **70+ commands** covering all common Frappe development workflows

## Installation

```bash
# Download the latest binary
curl -fsSL https://github.com/gavindsouza/weg/releases/latest/download/weg-$(uname -s)-$(uname -m) -o weg
chmod +x weg
sudo mv weg /usr/local/bin/

# Or build from source
git clone https://github.com/gavindsouza/weg
cd weg
go build -o weg .
```

## Quick Start

### Create a new Frappe app

```bash
# Create a new app with weg managing dependencies
weg new myapp
cd myapp

# Start development servers
weg start

# Open in browser (auto-login as Administrator)
weg site browse
```

### Work with an existing bench

```bash
# Initialize weg in an existing bench directory
cd /path/to/frappe-bench
weg init

# Start development
weg start
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
weg api call frappe.client.get_count --doctype User
weg doc get User Administrator   # Get a document
weg doc list User --limit 10     # List documents
weg doctype list                 # List all doctypes
```

### Frappe Cloud

```bash
weg cloud login                  # Authenticate with Frappe Cloud
weg cloud sites                  # List your sites
weg cloud deploy mysite          # Deploy to cloud
weg cloud logs mysite            # View site logs
```

## Configuration

Weg uses `weg.toml` for bench-centric projects or `pyproject.toml [tool.weg]` for app-centric projects.

### weg.toml (bench-centric)

```toml
[frappe]
version = "15"

[[apps]]
name = "erpnext"
url = "https://github.com/frappe/erpnext"
branch = "version-15"

[[sites]]
name = "mysite.localhost"
default = true
apps = ["frappe", "erpnext"]
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

## Shell Completions

```bash
# Bash
weg completion bash > /etc/bash_completion.d/weg

# Zsh
weg completion zsh > "${fpath[1]}/_weg"

# Fish
weg completion fish > ~/.config/fish/completions/weg.fish
```

## vs bench

| Feature | weg | bench |
|---------|-----|-------|
| Configuration | Declarative (TOML) | Imperative (commands) |
| Project structure | App-centric or bench-centric | Bench-centric only |
| Python management | uv (fast) | pip |
| System dependencies | devbox/Nix (reproducible) | Manual |
| Process management | process-compose | honcho/supervisord |
| API access | Direct (no HTTP) | Via HTTP |
| Cloud integration | Built-in | Separate tool |

## Requirements

- Go 1.21+ (for building from source)
- [devbox](https://www.jetpack.io/devbox) (installed automatically on first use)
- Git

## License

MIT
