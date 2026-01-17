# Requirements Checklist: 002-remote-clone

## Functional Requirements

### Core Operations
- [ ] **FR-001**: Clone remote site to git-backed directory with app/module structure
- [ ] **FR-002**: Bidirectional sync (pull and push)
- [ ] **FR-003**: Reconstruct git history from Version doctype on clone
- [ ] **FR-004**: Create Version records on remote when pushing
- [ ] **FR-005**: Detect and report conflicts

### Entity Support
- [ ] **FR-010**: Sync Custom DocTypes (`custom=1`)
- [ ] **FR-011**: Sync Custom Fields (`is_system_generated=0`)
- [ ] **FR-012**: Sync Property Setters
- [ ] **FR-013**: Sync Client Scripts
- [ ] **FR-014**: Sync Server Scripts
- [ ] **FR-015**: Sync custom Reports (`is_standard="No"`)
- [ ] **FR-016**: Sync custom Print Formats (`standard="No"`)
- [ ] **FR-017**: Sync Workflows (with States and Actions)
- [ ] **FR-018**: Sync Notifications
- [ ] **FR-019**: Sync Workspaces (configurable)
- [ ] **FR-020**: Sync Letter Heads
- [ ] **FR-021**: Sync Web Templates (custom)
- [ ] **FR-022**: Sync Number Cards, Dashboards, Dashboard Charts (configurable)

### Organization
- [ ] **FR-030**: Organize files by module (mirrors Frappe app structure)
- [ ] **FR-031**: `_/` directory for entities without module
- [ ] **FR-032**: Track module→app mapping in site.toml
- [ ] **FR-033**: Store sync settings in site.toml
- [ ] **FR-034**: Store credentials in gitignored credentials.toml

### File Formats
- [ ] **FR-040**: DocType files match Frappe native JSON format
- [ ] **FR-041**: Custom Fields grouped by target DocType
- [ ] **FR-042**: Scripts include metadata (frontmatter or sidecar)
- [ ] **FR-043**: Validate JSON/file syntax before push

### API & Discovery
- [ ] **FR-050**: Use Frappe REST API (no bench required)
- [ ] **FR-051**: Discover module-linked DocTypes via API
- [ ] **FR-052**: Support API key authentication
- [ ] **FR-053**: Support username/password authentication
- [ ] **FR-054**: Support OAuth authentication (future)

## Non-Functional Requirements

- [ ] **NFR-001**: Clone <100 customizations in <60 seconds
- [ ] **NFR-002**: Status check in <5 seconds
- [ ] **NFR-003**: Atomic push/pull (no partial changes on failure)
- [ ] **NFR-004**: Handle 1000+ customizations without memory issues
- [ ] **NFR-005**: Secure credential storage, never committed

## User Stories

### P0 (Must Have)
- [ ] **US-1**: Clone Remote Site
- [ ] **US-2**: Interactive Clone Configuration
- [ ] **US-3**: Pull Remote Changes
- [ ] **US-4**: Push Local Changes

### P1 (Should Have)
- [ ] **US-5**: Bidirectional Sync with Message
- [ ] **US-6**: View Sync Status
- [ ] **US-9**: Security Setup Guidance

### P2 (Nice to Have)
- [ ] **US-7**: Configure Sync Settings
- [ ] **US-8**: Convert to Frappe App

## Commands

### Implemented
- [ ] `weg clone <url> [dir]`
- [ ] `weg clone --modules=...`
- [ ] `weg clone --exclude=...`
- [ ] `weg pull`
- [ ] `weg push`
- [ ] `weg push --dry-run`
- [ ] `weg sync`
- [ ] `weg sync -m "message"`
- [ ] `weg status`
- [ ] `weg status --remote`
- [ ] `weg info`
- [ ] `weg modules`
- [ ] `weg entities`
- [ ] `weg config <key>`
- [ ] `weg config <key> <value>`
- [ ] `weg config --edit`
- [ ] `weg doctor --remote`
- [ ] `weg app create <name>`
- [ ] `weg app create --modules=...`

## Test Coverage

- [ ] Unit tests for TOML config parsing
- [ ] Unit tests for JSON file generation (Frappe format)
- [ ] Unit tests for Version→git commit translation
- [ ] Integration tests for clone flow
- [ ] Integration tests for pull/push/sync
- [ ] Integration tests for conflict detection
- [ ] E2E test against real Frappe site (CI)
