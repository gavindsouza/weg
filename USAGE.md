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

## API Commands

Make API calls to Frappe sites directly without HTTP. Executes as Administrator by default.

### Get Documents

```bash
# List all documents of a doctype
weg api get User
weg api get "Sales Invoice"

# Get specific document
weg api get User/Administrator
weg api get "Sales Invoice/INV-001"

# With filters and field selection
weg api get User --filters '{"enabled":1}' --fields '["name","email"]'
weg api get User --limit 10 --order-by "creation desc"
```

### Create Documents

```bash
# Create a new document
weg api post User -d '{"email":"test@example.com","first_name":"Test"}'
weg api post ToDo -d '{"description":"My task","priority":"High"}'
```

### Update Documents

```bash
# Update an existing document
weg api put User/test@example.com -d '{"first_name":"Updated Name"}'
weg api put "Sales Invoice/INV-001" -d '{"status":"Paid"}'
```

### Delete Documents

```bash
# Delete a document
weg api delete User/test@example.com
weg api delete "Sales Invoice/INV-001"
```

### Call Methods

```bash
# Call any frappe method
weg api call frappe.ping
weg api call frappe.utils.now

# With arguments (key=value or --args JSON)
weg api call frappe.client.get_count doctype=User
weg api call myapp.api.custom_function --args '{"param1":"value1"}'
```

### Run Document Methods

```bash
# Execute methods on specific documents (like doc.submit())
weg api run "Sales Invoice/INV-001" submit
weg api run "Sales Invoice/INV-001" cancel
weg api run User/Administrator get_fullname
weg api run ToDo/TODO-001 custom_method arg1=value1
```

### API Options

```bash
# Execute as different user
weg api get User --user Guest

# Target specific site
weg api get User --site mysite.localhost

# Raw JSON output (no formatting)
weg api get User --raw
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

## Upgrade Frappe Version

Upgrade to the next Frappe major version (e.g., 15 → 16 or 16 → develop).

```bash
# Upgrade to next version (auto-detected)
weg upgrade

# Skip database migrations
weg upgrade --no-migrate

# Auto-confirm
weg upgrade -y
```

The upgrade process:
1. Updates weg.toml configuration
2. Regenerates devbox environment (Python, Node versions)
3. Checks out new branch for frappe
4. Reinstalls Python dependencies
5. Reinstalls Node dependencies
6. Runs database migrations

## Configuration Commands

View and modify weg configuration settings.

```bash
# Show current configuration
weg config show

# Get specific value
weg config get frappe.version

# Set a value
weg config set frappe.version 15

# List configured apps
weg config list-apps
```

## Docker Commands

Manage Docker Compose deployments for local development or production.

```bash
# Generate docker-compose.yml
weg docker init

# Start containers
weg docker up
weg docker up -d              # Detached mode

# Stop containers
weg docker down

# View logs
weg docker logs
weg docker logs web           # Specific service

# List containers
weg docker ps
```

## Container Image Commands

Build and manage container images for Frappe deployments.

```bash
# Build container image
weg image build
weg image build --tag myapp:latest

# List images
weg image list

# Push to registry
weg image push myapp:latest
```

## Cloud Commands

Deploy and manage apps on Frappe Cloud.

```bash
# Authenticate with Frappe Cloud
weg cloud login

# List your sites
weg cloud sites

# List your benches
weg cloud benches

# Deploy to a site
weg cloud deploy mysite.frappe.cloud
weg cloud deploy --bench mybench

# Check deployment status
weg cloud status

# View site logs
weg cloud logs mysite.frappe.cloud

# Log out
weg cloud logout
```

## Migrate Project Structure

Convert between app-centric and bench-centric project layouts.

```bash
# Convert app-centric to bench-centric
weg migrate bench

# Convert bench-centric to app-centric
weg migrate app
```

App-centric mode keeps your app at the root with the bench hidden in `.weg/`.
Bench-centric mode uses the traditional Frappe bench structure with `apps/` and `sites/` at the root.

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
