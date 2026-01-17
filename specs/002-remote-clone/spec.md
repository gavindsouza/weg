# Feature Specification: weg clone - Remote Site Customization Sync

**Feature Branch**: `002-remote-clone`
**Created**: 2026-01-17
**Status**: Draft
**Input**: R&D exploration of remote site development flow - the third major weg workflow alongside app-centric and bench-centric development.

## Overview

`weg clone` enables developers to work with Frappe site customizations without direct bench/server access. It creates a git-backed local directory that mirrors the site's customization structure, enabling:

- Local file editing with AI assistance (Claude, etc.)
- Version control via git
- Bidirectional sync with remote site
- Team collaboration through git workflows
- Easy migration to proper Frappe apps

### The Three Weg Flows

| Flow | Command | Use Case |
|------|---------|----------|
| **App-Centric** | `weg init .` (in app dir) | Developing a Frappe app, bench hidden in `.weg/` |
| **Bench-Centric** | `weg init` (new bench) | Traditional bench workflow with `weg.toml` |
| **Remote-Site** | `weg clone <url>` | Working with sites without bench access (Frappe Cloud, client servers) |

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Clone Remote Site (Priority: P0)

A developer wants to export all customizations from a remote Frappe site to a local git-backed directory so they can edit files locally and version control their work.

**Why this priority**: This is the foundational capability. Without cloning, no remote development workflow is possible.

**Independent Test**: Run `weg clone <url>` against a site with customizations and verify the directory structure is created with correct files and git history.

**Acceptance Scenarios**:

1. **Given** a Frappe site URL and valid API credentials, **When** user runs `weg clone https://mysite.frappe.cloud`, **Then** a `mysite/` directory is created with `.weg/site.toml`, `modules.txt`, and module directories containing customization files
2. **Given** a site with 3 Custom DocTypes in module "Custom", **When** clone completes, **Then** `custom/doctype/` contains 3 subdirectories with `.json` files matching Frappe's export format
3. **Given** a site with Version history for customizations, **When** clone completes, **Then** git log shows commits reconstructed from Version records with matching timestamps and descriptions
4. **Given** Client Scripts without a module assigned, **When** clone completes, **Then** scripts appear in `_/client_script/` (catch-all module)
5. **Given** Custom Fields with `is_system_generated=1`, **When** clone completes, **Then** those fields are excluded (only user-created fields synced)

---

### User Story 2 - Interactive Clone Configuration (Priority: P0)

A developer wants to choose which entity types and modules to sync during clone, so they don't pull unnecessary data.

**Why this priority**: Sites may have hundreds of customizations across many modules. Developers need control over scope.

**Independent Test**: Run `weg clone <url>` and verify interactive prompts allow selecting/deselecting entity types and modules.

**Acceptance Scenarios**:

1. **Given** a site with Workspaces and Notifications, **When** user deselects these during interactive setup, **Then** `.weg/site.toml` shows `workspace = false` and `notification = false`, and those directories are not created
2. **Given** multiple modules on site, **When** user selects only "Custom" and "Selling", **Then** only those module directories are created and `modules.txt` contains only those two
3. **Given** `weg clone <url> --modules=Custom,Selling`, **When** command runs, **Then** interactive prompts are skipped for module selection and only specified modules are cloned
4. **Given** `weg clone <url> --exclude=workspace,notification`, **When** command runs, **Then** those entity types are excluded without prompting

---

### User Story 3 - Pull Remote Changes (Priority: P0)

A developer wants to fetch changes made on the remote site (by other users via Desk UI) and merge them into their local files.

**Why this priority**: Bidirectional sync requires both push and pull. Pull ensures local stays in sync with remote.

**Independent Test**: Make changes on remote site via Desk, run `weg pull`, verify local files updated and git commits created.

**Acceptance Scenarios**:

1. **Given** a remote Custom Field was added since last sync, **When** user runs `weg pull`, **Then** the field appears in the appropriate `custom_field/*.json` file and a git commit is created with the Version description
2. **Given** no changes on remote since last sync, **When** user runs `weg pull`, **Then** output shows "Already up to date" and no commits are created
3. **Given** a remote Script was modified, **When** user runs `weg pull`, **Then** the local file is updated, preserving any non-conflicting local changes (fast-forward merge)
4. **Given** the same file was modified both locally and remotely, **When** user runs `weg pull`, **Then** conflict markers are inserted and user is prompted to resolve before sync can continue

---

### User Story 4 - Push Local Changes (Priority: P0)

A developer (or AI assistant) has modified local files and wants to apply those changes to the remote Frappe site.

**Why this priority**: Push completes the sync loop, making local development effective.

**Independent Test**: Modify a local file, run `weg push`, verify change appears on remote site.

**Acceptance Scenarios**:

1. **Given** a modified doctype JSON with a new field, **When** user runs `weg push`, **Then** the doctype on remote site is updated with the new field and a Version record is created
2. **Given** a new Client Script file created locally, **When** user runs `weg push`, **Then** the script is created on the remote site
3. **Given** invalid JSON syntax in a file, **When** user runs `weg push`, **Then** an error identifies the file and problem, and no changes are made to remote
4. **Given** uncommitted local changes, **When** user runs `weg push`, **Then** user is prompted to commit first (or use `weg sync -m "message"`)
5. **Given** `weg push --dry-run`, **When** command runs, **Then** shows what would be pushed without making changes

---

### User Story 5 - Bidirectional Sync with Message (Priority: P1)

A developer wants to pull remote changes and push local changes in one command, with a descriptive message for their changes.

**Why this priority**: Most common workflow - make changes, sync with message.

**Independent Test**: Make local changes, have remote changes pending, run `weg sync -m "message"`, verify both directions sync.

**Acceptance Scenarios**:

1. **Given** local changes and remote changes with no conflicts, **When** user runs `weg sync -m "Add priority field"`, **Then** remote changes are pulled first, local changes are committed with the message, then pushed to remote
2. **Given** only local changes, **When** user runs `weg sync -m "Fix validation"`, **Then** changes are committed and pushed, Version on remote has the same description
3. **Given** only remote changes, **When** user runs `weg sync`, **Then** remote changes are pulled, no commit message needed
4. **Given** conflicts exist, **When** user runs `weg sync`, **Then** pull stops at conflict, user resolves, then runs `weg sync` again to complete

---

### User Story 6 - View Sync Status (Priority: P1)

A developer wants to see what's different between local files and remote site before syncing.

**Why this priority**: Essential for safe development - know what will change before changing it.

**Independent Test**: Make local and remote changes, run `weg status`, verify accurate diff summary.

**Acceptance Scenarios**:

1. **Given** local file modifications, **When** user runs `weg status`, **Then** output shows files as "modified locally" with summary of changes
2. **Given** remote changes since last sync, **When** user runs `weg status`, **Then** output shows entities as "modified remotely" with change summary
3. **Given** new local file not on remote, **When** user runs `weg status`, **Then** output shows file as "new (local only)"
4. **Given** entity on remote not in local files, **When** user runs `weg status`, **Then** output shows as "new (remote only)"
5. **Given** everything in sync, **When** user runs `weg status`, **Then** output shows "Everything up to date"

---

### User Story 7 - Configure Sync Settings (Priority: P2)

A developer wants to modify sync settings after initial clone (enable/disable entity types, exclude patterns).

**Why this priority**: Requirements change over time; settings should be adjustable.

**Independent Test**: Run `weg config` commands, verify `.weg/site.toml` is updated and next sync respects changes.

**Acceptance Scenarios**:

1. **Given** workspaces are currently synced, **When** user runs `weg config sync.entities.workspace false`, **Then** `.weg/site.toml` is updated and next pull skips workspaces
2. **Given** user wants to exclude HR customizations, **When** user runs `weg config sync.exclude.patterns --add "hr/*"`, **Then** pattern is added and matching files are ignored on sync
3. **Given** `weg config --edit`, **When** command runs, **Then** `.weg/site.toml` opens in `$EDITOR`
4. **Given** `weg config sync.entities`, **When** command runs, **Then** all entity type settings are displayed

---

### User Story 8 - Convert to Frappe App (Priority: P2)

A developer has accumulated customizations and wants to export them as a proper Frappe app for better maintainability and deployment.

**Why this priority**: Natural progression from customizations to proper app development.

**Independent Test**: Run `weg app create`, verify valid Frappe app structure is created.

**Acceptance Scenarios**:

1. **Given** customizations in multiple modules, **When** user runs `weg app create my_customizations`, **Then** a Frappe app scaffold is created with `hooks.py`, `pyproject.toml`, and module directories containing the customizations
2. **Given** `weg app create my_app --modules=Custom`, **When** command runs, **Then** only Custom module customizations are included in the app
3. **Given** Custom Fields in the clone, **When** app is created, **Then** fields are exported as fixtures with proper `hooks.py` fixture configuration
4. **Given** the created app, **When** installed on a bench via `weg app get ./my_customizations`, **Then** all customizations are applied to the site

---

### User Story 9 - Security Setup Guidance (Priority: P1)

A developer needs clear guidance on setting up secure API access for remote sync.

**Why this priority**: Security is critical - improper setup could expose site to unauthorized modifications.

**Independent Test**: Run `weg clone` without credentials, verify security guidance is displayed.

**Acceptance Scenarios**:

1. **Given** first-time clone attempt, **When** no credentials provided, **Then** detailed security setup instructions are displayed explaining the "Weg Sync" role and required permissions
2. **Given** credentials via environment variables `WEG_API_KEY` and `WEG_API_SECRET`, **When** `weg clone` runs, **Then** credentials are used without prompting
3. **Given** credentials in `.weg/credentials.toml`, **When** file exists, **Then** it is automatically gitignored and credentials are loaded from it
4. **Given** `weg doctor --remote`, **When** command runs, **Then** connection is tested and permission issues are diagnosed with specific remediation steps

---

### Edge Cases

- **Site unreachable during sync**: Error with retry suggestion, no partial changes applied locally or remotely
- **API credentials expired**: Clear error message with re-authentication instructions
- **Frappe version mismatch**: Warning if remote Frappe version differs significantly from when cloned; suggest re-clone if major version change
- **Module deleted on remote**: Warning shown, local module directory preserved with note
- **Concurrent pushes from multiple clones**: Last-write-wins with warning; recommend git-based coordination
- **Very large site (1000+ customizations)**: Progress indicators, chunked API calls, resumable clone
- **Entity references missing target**: Validation error with clear message (e.g., Link field pointing to non-existent DocType)

## Requirements *(mandatory)*

### Functional Requirements

#### Core Operations
- **FR-001**: System MUST clone remote site customizations to a local git-backed directory structure that mirrors Frappe's app/module organization
- **FR-002**: System MUST support bidirectional sync: pull (remote→local) and push (local→remote)
- **FR-003**: System MUST reconstruct git history from Frappe Version doctype records during initial clone
- **FR-004**: System MUST create Version records on remote when pushing local changes
- **FR-005**: System MUST detect and report conflicts when same entity modified both locally and remotely

#### Entity Support
- **FR-010**: System MUST sync Custom DocTypes (`DocType` where `custom=1`)
- **FR-011**: System MUST sync Custom Fields (`Custom Field` where `is_system_generated=0`)
- **FR-012**: System MUST sync Property Setters
- **FR-013**: System MUST sync Client Scripts
- **FR-014**: System MUST sync Server Scripts
- **FR-015**: System MUST sync custom Reports (`Report` where `is_standard="No"`)
- **FR-016**: System MUST sync custom Print Formats (`Print Format` where `standard="No"`)
- **FR-017**: System MUST sync Workflows (including Workflow States and Actions)
- **FR-018**: System MUST sync Notifications
- **FR-019**: System SHOULD sync Workspaces (configurable, default off)
- **FR-020**: System SHOULD sync Letter Heads
- **FR-021**: System SHOULD sync Web Templates (custom only)
- **FR-022**: System SHOULD sync Number Cards, Dashboards, Dashboard Charts (configurable)

#### Organization
- **FR-030**: System MUST organize files by module, mirroring Frappe app structure
- **FR-031**: System MUST use `_/` directory as catch-all for entities without module assigned
- **FR-032**: System MUST track module→app mapping in `.weg/site.toml`
- **FR-033**: System MUST store sync settings (entity types, exclusions) in `.weg/site.toml`
- **FR-034**: System MUST store credentials separately in gitignored `.weg/credentials.toml`

#### File Formats
- **FR-040**: DocType files MUST match Frappe's native JSON export format for compatibility
- **FR-041**: Custom Fields MUST be grouped by target DocType in single JSON files
- **FR-042**: Scripts MUST include metadata (doctype, event, module) in frontmatter or sidecar file
- **FR-043**: System MUST validate JSON/file syntax before pushing to remote

#### API & Discovery
- **FR-050**: System MUST use Frappe REST API for all remote operations (no bench access required)
- **FR-051**: System MUST discover module-linked DocTypes via API (`get_meta` with module Link field check)
- **FR-052**: System MUST support API key authentication
- **FR-053**: System SHOULD support username/password authentication
- **FR-054**: System SHOULD support OAuth authentication (future)

### Non-Functional Requirements

- **NFR-001**: Clone operation MUST complete within 60 seconds for sites with <100 customizations
- **NFR-002**: Sync status check MUST complete within 5 seconds
- **NFR-003**: Push/pull operations MUST be atomic - no partial changes on failure
- **NFR-004**: System MUST handle sites with 1000+ customizations without memory issues
- **NFR-005**: All credentials MUST be stored securely and never committed to git

### Key Entities

- **SiteConfig** (`.weg/site.toml`): Site URL, app versions, module mappings, sync settings, version tracking
- **Credentials** (`.weg/credentials.toml`): API keys/secrets, gitignored
- **Module**: Directory representing a Frappe module, contains entity subdirectories
- **CatchAllModule** (`_/`): Special module for entities without module assigned
- **SyncManifest**: Tracks last sync state, file→Version mappings, checksums

## File Structure

```
mysite/                              # Clone root (git repo)
├── .git/
├── .weg/
│   ├── site.toml                    # Site metadata + sync settings (committed)
│   └── credentials.toml             # Auth secrets (gitignored)
│
├── modules.txt                      # List of synced modules
│
├── _/                               # Catch-all module (no module assigned)
│   ├── client_script/
│   │   └── global_helpers.json      # Script definition
│   └── server_script/
│       └── api_logger.json
│
├── custom/                          # Module: Custom (app: _site)
│   ├── doctype/
│   │   └── todo_item/
│   │       ├── todo_item.json       # DocType definition (Frappe format)
│   │       ├── todo_item.py         # Controller (if exists)
│   │       └── todo_item.js         # Form script (if exists)
│   │
│   ├── report/
│   │   └── sales_summary/
│   │       ├── sales_summary.json   # Report metadata
│   │       ├── sales_summary.py     # Query script
│   │       └── sales_summary.js     # Client script (optional)
│   │
│   ├── client_script/
│   │   └── sales_invoice.json       # Script with dt, script fields
│   │
│   ├── server_script/
│   │   └── validate_stock.json
│   │
│   ├── print_format/
│   │   └── custom_invoice.json      # Includes HTML in field
│   │
│   ├── workflow/
│   │   └── expense_approval.json    # Includes states + actions
│   │
│   └── notification/
│       └── low_stock_alert.json
│
├── selling/                         # Module: Selling (app: erpnext)
│   ├── custom_field/
│   │   └── sales_invoice.json       # All custom fields for this doctype
│   │
│   └── property_setter/
│       └── sales_invoice.json       # All property setters for this doctype
│
└── accounts/                        # Module: Accounts (app: erpnext)
    └── custom_field/
        └── journal_entry.json
```

## Configuration Schema

```toml
# .weg/site.toml

[site]
url = "https://mysite.frappe.cloud"
name = "mysite"
cloned_at = "2026-01-17T12:30:00Z"

[site.auth]
method = "api_key"  # api_key | password | oauth

[site.frappe]
version = "16.0.0"

[site.apps]
frappe = { version = "16.0.0" }
erpnext = { version = "16.0.0" }
hrms = { version = "16.0.0" }

[modules]
# module = { app = "app_name", sync = bool }
Core = { app = "frappe", sync = false }
Custom = { app = "_site", sync = true }
Selling = { app = "erpnext", sync = true }
"_" = { app = "_site", sync = true }  # Catch-all

[sync]
last_sync = "2026-01-17T14:45:00Z"

[sync.entities]
doctype = true
custom_field = true
property_setter = true
client_script = true
server_script = true
report = true
print_format = true
workspace = false
notification = false
workflow = true
letter_head = true
web_template = false
number_card = false
dashboard = false
dashboard_chart = false

[sync.exclude]
patterns = [
    "*/doctype/test_*",
    "hr/*",
]

[versions]
# file_path = { version = "VER-xxx", modified = "timestamp" }
"custom/doctype/todo_item/todo_item.json" = { version = "VER-00045", modified = "2026-01-15T10:00:00Z" }
```

## Command Reference

```bash
# Clone
weg clone <url> [directory]
weg clone <url> --modules=Custom,Selling
weg clone <url> --exclude=workspace,notification
weg clone <url> --non-interactive

# Sync
weg pull                    # Remote → Local
weg push                    # Local → Remote
weg push --dry-run          # Preview push
weg sync                    # Bidirectional
weg sync -m "message"       # With commit message

# Status
weg status                  # Local vs remote diff
weg status --remote         # Fetch fresh remote state

# Info
weg info                    # Site info, app versions
weg modules                 # List modules + sync status
weg entities                # Entity types + counts

# Config
weg config <key>            # Get value
weg config <key> <value>    # Set value
weg config --edit           # Edit site.toml

# Diagnostics
weg doctor --remote         # Test connection, permissions

# Convert to App
weg app create <name>
weg app create <name> --modules=Custom
weg app create <name> --from=.
```

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Developer can clone a remote site and have working local files within 2 minutes
- **SC-002**: AI assistants (Claude, etc.) can modify customizations by editing local JSON/Python/JS files
- **SC-003**: Full git history is available showing all customization changes with original timestamps
- **SC-004**: Round-trip sync (clone → modify → push → pull on another machine) produces identical results
- **SC-005**: Converting to Frappe app produces installable app that passes `bench build`
- **SC-006**: 100% of validation errors caught before any remote changes (atomic operations)
- **SC-007**: Team of 3 developers can collaborate on same site's customizations via git workflow

## Security Considerations

### Required Permissions

The sync user needs permissions on these DocTypes:

| DocType | Permissions |
|---------|------------|
| Custom Field | read, write, create, delete |
| Property Setter | read, write, create, delete |
| Client Script | read, write, create, delete |
| Server Script | read, write, create, delete |
| Report (is_standard=No) | read, write, create, delete |
| Print Format (standard=No) | read, write, create, delete |
| DocType (custom=1) | read, write, create, delete |
| Workflow | read, write, create, delete |
| Notification | read, write, create, delete |
| Version | read |
| Module Def | read |

### Recommended Setup

1. Create Role "Weg Sync" with above permissions
2. Create dedicated User with only "Weg Sync" role
3. Generate API key for that user
4. Store credentials in environment variables or gitignored file

### Security Warnings

- **Never commit credentials** - `.weg/credentials.toml` must be gitignored
- **Audit API access** - Sync user actions are logged in Frappe's activity log
- **Limit scope** - Only grant permissions for entity types being synced
- **Rotate keys** - Periodically regenerate API keys

## Assumptions

- Remote site has REST API enabled (standard in Frappe 14+)
- User has or can create API credentials with sufficient permissions
- Site's Frappe version is 14.0.0 or higher
- Network connectivity is available during sync operations
- Git is installed on the developer's machine
- The `.weg/` directory structure is compatible with future `weg custom` (001) integration

## Future Considerations

- **Webhook-based sync**: Real-time push on remote changes instead of polling
- **Conflict resolution UI**: Visual diff/merge tool for conflicts
- **Selective entity sync**: Push/pull specific files only
- **Multi-site management**: Clone and sync multiple sites from one workspace
- **Integration with 001-custom-command**: Unified local development when bench available
