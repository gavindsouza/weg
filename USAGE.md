# Weg Usage Guide

**Binary location**: `/home/gavin/Desktop/projects/weg/weg-test`

Weg is a modern CLI for managing Frappe development environments. It replaces the traditional `bench` CLI with faster, declarative tooling.

## Creating a New App

```bash
# Create new app in new directory
weg new my-awesome-app

# Create app in current directory (existing content preserved)
weg new .

# With options
weg new my-app --version 15 --database mariadb

# Non-interactive (use defaults)
weg new my-app -y
```

This creates:
- `my_app/` module with hooks.py, __init__.py
- `pyproject.toml` with [tool.weg] config
- `.weg/` development environment
- README.md, .gitignore

## Quick Reference

```bash
# Alias for convenience (optional)
alias weg='/home/gavin/Desktop/projects/weg/weg-test'

# Or use full path
/home/gavin/Desktop/projects/weg/weg-test <command>
```

## Core Commands

### Create a New Bench (Recommended - Works Now)

```bash
# Create a new Frappe bench with specific version
weg create <path> --version <14|15|16> --database <mariadb|postgres>

# Example: Create v15 bench with MariaDB
weg create ~/projects/my-frappe-project --version 15 --database mariadb

# With specific apps
weg create ~/myproject --apps '[{"Url": "https://github.com/frappe/erpnext", "Branch": "version-15"}]' --version 15
```

### Initialize Weg in Existing Directory

```bash
cd /path/to/your/project
weg init

# Creates weg.toml (config) and prepares .weg/ directory
# Follow prompts for Frappe version and database
```

### Check Status

```bash
# In project directory
weg status

# From anywhere using -C flag
weg -C /path/to/project status
```

## App Management

```bash
# List apps in project
weg app list

# Add an app
weg app get https://github.com/frappe/erpnext version-15

# Remove an app
weg app remove erpnext

# Switch app branch
weg app switch erpnext version-14
```

## Site Management

```bash
# List sites
weg site list

# Create new site
weg site new mysite.localhost

# Set default site
weg site use mysite.localhost

# Install app on site
weg site install mysite.localhost erpnext

# Delete site
weg site drop mysite.localhost
```

## Development Workflow

```bash
# Start development server (via process-compose)
weg start

# Stop services
weg stop

# View logs
weg logs
weg logs web
weg logs worker

# Build frontend assets
weg build
weg build --watch
weg build --production
```

## Running Commands in Bench Context

```bash
# Run any command in the bench environment
weg exec -- <command>

# Shortcuts for common operations
weg exec migrate          # bench --site <default> migrate
weg exec console          # bench --site <default> console
weg exec mariadb          # bench --site <default> mariadb
weg exec backup           # bench --site <default> backup

# With specific site
weg exec --site mysite.localhost migrate
```

## Sync Configuration

```bash
# Preview changes
weg sync --dry-run

# Apply configuration changes
weg sync

# Auto-confirm
weg sync --yes
```

## Update Apps

```bash
# Update all apps
weg update

# Update specific app
weg update erpnext

# Only pull, skip deps
weg update --pull

# Skip asset rebuild
weg update --no-build
```

## Global Flags

All commands support these flags:

```bash
-C, --chdir <path>    # Run as if started in <path> (like git -C)
-v, --verbose         # Enable verbose output
-q, --quiet           # Suppress non-essential output
-y, --yes             # Auto-confirm prompts
--config <path>       # Custom config file path
```

## Configuration Files

### weg.toml (Bench Configuration)

Located at project root for bench-style projects:

```toml
[bench]
name = "my-project"

[frappe]
version = "15"
database = "mariadb"

[apps.frappe]
url = "https://github.com/frappe/frappe"
branch = "version-15"

[apps.erpnext]
url = "https://github.com/frappe/erpnext"
branch = "version-15"

[[sites]]
name = "mysite.localhost"
default = true
apps = ["frappe", "erpnext"]
```

### pyproject.toml [tool.weg] (App Configuration)

For app-centric development (your app is the project root):

```toml
[tool.weg]
# Compatible Frappe versions
compatibility.frappe = ["14", "15", "16"]
compatibility.databases = ["mariadb", "postgres"]

[tool.weg.dev]
frappe_version = "15"
database = "mariadb"
site_name = "myapp.localhost"

[tool.weg.dependencies]
# Additional apps needed for development
erpnext = { url = "https://github.com/frappe/erpnext", branch = "version-15" }
```

## Project Structure

### App-Centric (Recommended)
```
my-frappe-app/
├── my_frappe_app/
│   ├── __init__.py
│   ├── hooks.py
│   └── ...
├── pyproject.toml      # With [tool.weg] section
├── .weg/               # Hidden bench (auto-created)
│   ├── apps/
│   ├── sites/
│   └── state.json
└── ...
```

### Bench-Style (Traditional)
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

## Examples

### Setting Up a New ERPNext Project

```bash
# Create the project
weg create ~/projects/erpnext-dev --version 15 --database mariadb

# Navigate to it
cd ~/projects/erpnext-dev

# Check status
weg status

# Add ERPNext
weg app get https://github.com/frappe/erpnext version-15

# Create a site
weg site new erp.localhost

# Install ERPNext on the site
weg site install erp.localhost erpnext

# Start the server
weg start
```

### Working with an Existing Frappe App

```bash
# Clone your app
git clone https://github.com/your-org/your-app
cd your-app

# Initialize weg
weg init
# Select Frappe version: 15
# Select database: mariadb

# Sync to set up environment
weg sync --yes

# Start developing
weg start
```

## Troubleshooting

### "devbox not found" or "uv not found"

Install required tools:
```bash
weg self install-tools
```

### "not a weg-managed project"

Run `weg init` in the project directory first, or use `weg create` to start fresh.

### Using from Anywhere

Use the `-C` flag:
```bash
weg -C /path/to/project status
weg -C /path/to/project app list
```

## For Claude/AI Assistants

When helping users with Frappe development using weg:

1. **Binary path**: `/home/gavin/Desktop/projects/weg/weg-test`
2. **Prefer `weg create`** for new projects - it's fully functional
3. **Use `-C` flag** to operate on projects without cd'ing
4. **Check `weg status`** to understand project state
5. **Config files**: Look for `weg.toml` or `pyproject.toml [tool.weg]`
6. **State tracking**: `.weg/state.json` tracks what's installed

Common workflow:
```bash
# Create project
/home/gavin/Desktop/projects/weg/weg-test create /path/to/new-project --version 15

# Check status
/home/gavin/Desktop/projects/weg/weg-test -C /path/to/new-project status

# Run bench commands
/home/gavin/Desktop/projects/weg/weg-test -C /path/to/new-project exec migrate
```
