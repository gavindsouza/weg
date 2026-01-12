# Feature Specification: weg custom - CLI-First Frappe Customization Development

**Feature Branch**: `001-custom-command`
**Created**: 2026-01-12
**Status**: Draft
**Input**: User description: "weg custom - CLI-first Frappe customization development. A command namespace for managing site customizations (doctypes, fields, scripts) through the CLI instead of Desk UI."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Pull Customizations to Disk (Priority: P1)

A developer wants to export all existing site customizations to local YAML files so they can version control them and edit them with their preferred tools (or have an LLM assist).

**Why this priority**: This is the foundational capability - without pulling customizations to disk, no other CLI-based development workflow is possible. It enables the entire "files as source of truth" paradigm.

**Independent Test**: Can be fully tested by running `weg custom pull` against a site with existing Custom Doctypes/Scripts and verifying YAML files appear in `.weg/custom/` directory with correct structure.

**Acceptance Scenarios**:

1. **Given** a Frappe site with 3 Custom Doctypes, **When** user runs `weg custom pull`, **Then** 3 YAML files are created in `.weg/custom/doctypes/` with complete doctype definitions
2. **Given** a site with Client Scripts and Server Scripts, **When** user runs `weg custom pull`, **Then** script files are created in `.weg/custom/scripts/client/` and `.weg/custom/scripts/server/` respectively
3. **Given** an empty `.weg/custom/` directory, **When** user runs `weg custom pull` on a site with no customizations, **Then** the command completes successfully with a message indicating no customizations found

---

### User Story 2 - Push Local Changes to Site (Priority: P1)

A developer (or LLM) has created or modified YAML files on disk and wants to apply those changes to the Frappe site.

**Why this priority**: Equal priority with pull - together they form the core sync loop. Without push, disk-based development has no effect on the running site.

**Independent Test**: Can be tested by modifying a YAML file (adding a field), running `weg custom push`, and verifying the change appears in the site's doctype.

**Acceptance Scenarios**:

1. **Given** a modified doctype YAML with a new field, **When** user runs `weg custom push`, **Then** the doctype on the site is updated with the new field
2. **Given** a new doctype YAML file that doesn't exist on site, **When** user runs `weg custom push`, **Then** the doctype is created on the site
3. **Given** invalid YAML syntax in a file, **When** user runs `weg custom push`, **Then** an error message identifies the file and line with the syntax error
4. **Given** a YAML file with invalid Frappe schema (e.g., unknown fieldtype), **When** user runs `weg custom push`, **Then** an error message explains the validation failure before any changes are made

---

### User Story 3 - Create New Doctype via CLI (Priority: P2)

A developer wants to quickly scaffold a new Custom Doctype with fields without opening the Desk UI, using CLI flags to specify all meta options.

**Why this priority**: Accelerates the most common customization task. High value for LLM-assisted development where CLI commands are easier to generate than UI interactions.

**Independent Test**: Can be tested by running `weg custom doctype new` with various flags and verifying the YAML file is created with correct structure.

**Acceptance Scenarios**:

1. **Given** the command `weg custom doctype new TodoItem --module=Custom`, **When** executed, **Then** a YAML file is created at `.weg/custom/doctypes/todo_item.yaml` with the doctype skeleton
2. **Given** the command with flags `--is-submittable --track-changes --naming-rule=autoincrement`, **When** executed, **Then** the YAML file includes these meta properties correctly set
3. **Given** a doctype name that already exists in `.weg/custom/doctypes/`, **When** user runs `weg custom doctype new` with that name, **Then** an error is shown asking to use `weg custom doctype get` instead

---

### User Story 4 - Add Field to Doctype via CLI (Priority: P2)

A developer wants to add a new field to an existing doctype using CLI kwargs that map to all Frappe field meta options.

**Why this priority**: Field manipulation is the most frequent customization operation. CLI kwargs provide faster iteration than editing YAML manually for simple additions.

**Independent Test**: Can be tested by running `weg custom field add` and verifying the field appears in the doctype YAML with correct properties.

**Acceptance Scenarios**:

1. **Given** an existing doctype YAML, **When** user runs `weg custom field add TodoItem priority --type=Select --options="Low\nMedium\nHigh"`, **Then** the field is appended to the doctype's fields array
2. **Given** field flags `--reqd --in-list-view --default=Medium`, **When** command is executed, **Then** the field YAML includes `reqd: true`, `in_list_view: true`, and `default: Medium`
3. **Given** an unknown field type `--type=InvalidType`, **When** command is executed, **Then** an error lists valid Frappe fieldtypes
4. **Given** a Link field `--type=Link --options=User`, **When** command is executed, **Then** the field includes proper `options: User` for the link target

---

### User Story 5 - View Sync Status (Priority: P2)

A developer wants to see what has changed between their local files and the site before pushing, similar to `git status`.

**Why this priority**: Essential for safe development workflow - prevents accidental overwrites and helps developers understand current state.

**Independent Test**: Can be tested by modifying local files and running `weg custom status` to see diff summary.

**Acceptance Scenarios**:

1. **Given** local files matching site state, **When** user runs `weg custom status`, **Then** output shows "No changes" or lists synced items
2. **Given** a locally modified doctype, **When** user runs `weg custom status`, **Then** output shows the doctype as "modified" with summary of changes
3. **Given** a new local doctype file not on site, **When** user runs `weg custom status`, **Then** output shows the doctype as "new (local only)"
4. **Given** a site doctype not in local files, **When** user runs `weg custom status`, **Then** output shows the doctype as "remote only"

---

### User Story 6 - Preview Doctype Form Layout (Priority: P3)

A developer wants to visualize how a doctype form will look without opening a browser, useful for LLMs to "see" what they're building.

**Why this priority**: Nice-to-have that improves developer experience and LLM feedback loops, but not essential for core sync workflow.

**Independent Test**: Can be tested by running `weg custom doctype preview` and verifying ASCII form rendering matches field definitions.

**Acceptance Scenarios**:

1. **Given** a doctype with Data, Select, and Link fields, **When** user runs `weg custom doctype preview TodoItem`, **Then** an ASCII/TUI representation shows field labels and types in form layout
2. **Given** a doctype with Section Breaks, **When** preview is run, **Then** sections are visually separated in the output
3. **Given** required fields in the doctype, **When** preview is run, **Then** required fields are marked with an asterisk or similar indicator

---

### User Story 7 - Watch Mode for Auto-Sync (Priority: P3)

A developer wants file changes to automatically push to the site for rapid iteration, especially useful when an LLM is making sequential edits.

**Why this priority**: Productivity enhancement for power users - reduces manual push commands during active development.

**Independent Test**: Can be tested by starting watch mode, editing a YAML file, and verifying the change appears on site without manual push.

**Acceptance Scenarios**:

1. **Given** watch mode is started with `weg custom watch`, **When** a doctype YAML file is saved, **Then** changes are automatically pushed to the site within 2 seconds
2. **Given** watch mode is running, **When** a syntax error is introduced, **Then** an error is displayed but watch mode continues running
3. **Given** watch mode is running, **When** user presses Ctrl+C, **Then** watch mode stops gracefully

---

### User Story 8 - Migrate Customizations to App (Priority: P3)

A developer has accumulated many customizations and wants to export them into a proper Frappe app scaffold for better maintainability.

**Why this priority**: Important for long-term sustainability but typically done less frequently than daily development operations.

**Independent Test**: Can be tested by running migration command and verifying a valid Frappe app structure is created with the customizations as fixtures.

**Acceptance Scenarios**:

1. **Given** customizations in `.weg/custom/`, **When** user runs `weg custom migrate --app=my_customizations`, **Then** a Frappe app scaffold is created in `apps/my_customizations/` with the customizations as fixtures
2. **Given** specific doctypes to include, **When** user runs `weg custom migrate --app=todo_app --include=TodoItem,TodoList`, **Then** only the specified doctypes are included in the app
3. **Given** the migration command, **When** executed, **Then** the app includes proper `__init__.py`, `hooks.py`, and fixture loading setup

---

### Edge Cases

- What happens when site is unreachable during push? Error with retry suggestion, no partial changes applied.
- What happens when two developers push conflicting changes? Last-write-wins with warning; recommend using version control for coordination.
- What happens when a doctype on site has been modified since last pull? Warning shown with option to force push or pull first.
- What happens when YAML references a Link target doctype that doesn't exist? Validation error during push with clear message.
- What happens when field is removed from YAML? Field is removed from site doctype on push (with confirmation prompt for destructive changes).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST store customizations as YAML files in `.weg/custom/` directory with subdirectories for `doctypes/` and `scripts/client/` and `scripts/server/`
- **FR-002**: System MUST convert YAML to JSON format when communicating with Frappe site API
- **FR-003**: System MUST support all Frappe doctype meta options as CLI flags (e.g., `--is-submittable`, `--track-changes`, `--naming-rule`)
- **FR-004**: System MUST support all Frappe field meta options as CLI flags (e.g., `--type`, `--options`, `--reqd`, `--read-only`, `--depends-on`, `--fetch-from`, `--in-list-view`)
- **FR-005**: System MUST validate YAML syntax before attempting to push to site
- **FR-006**: System MUST validate Frappe schema requirements (valid fieldtypes, required fields) before pushing
- **FR-007**: System MUST provide clear error messages identifying file, line number, and specific validation failure
- **FR-008**: System MUST support selective push/pull of specific doctypes or scripts by name
- **FR-009**: System MUST track sync state to detect local vs remote changes
- **FR-010**: System MUST prompt for confirmation before destructive operations (field removal, doctype deletion)
- **FR-011**: System MUST support dry-run mode (`--dry-run`) to preview changes without applying them

### Key Entities

- **CustomDoctype**: Represents a Custom Doctype definition including meta options (module, naming, permissions) and fields array. Stored as YAML, converted to JSON for Frappe API.
- **CustomField**: A field within a doctype with all meta properties (fieldname, fieldtype, label, options, etc.). Nested within doctype YAML.
- **ClientScript**: JavaScript code that runs in the browser for a specific doctype. Stored as `.js` file with metadata header.
- **ServerScript**: Python code that runs on the server triggered by doctype events. Stored as `.py` file with metadata header.
- **SyncManifest**: Tracks last sync timestamps and checksums for each customization to detect drift between local and remote.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Developers can create a new doctype with 5 fields in under 60 seconds using CLI commands only (no browser)
- **SC-002**: LLM-assisted development can modify customizations by editing YAML files directly, with changes reflected on site after push
- **SC-003**: All Frappe doctype and field meta options are accessible via CLI kwargs with no gaps in coverage
- **SC-004**: Sync status command provides complete visibility into local vs remote state in under 2 seconds
- **SC-005**: Push operations complete within 5 seconds for typical customizations (under 10 doctypes)
- **SC-006**: 100% of validation errors are caught before any changes are made to the site (atomic operations)
- **SC-007**: Migration to app produces a valid, installable Frappe app that passes `bench build` without errors

## Assumptions

- The Frappe site has API access enabled and appropriate user credentials are configured in weg
- Custom Doctypes are created within the "Custom" module by default unless specified otherwise
- File naming follows snake_case convention derived from doctype name (e.g., TodoItem -> todo_item.yaml)
- Scripts reference their target doctype in a metadata header comment
- The `.weg/custom/` directory is intended to be version-controlled alongside the project
