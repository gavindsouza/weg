package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/spf13/cobra"
)

var scaffoldCmd = &cobra.Command{
	Use:   "scaffold [type]",
	Short: "Scaffold development tooling into your project",
	Long: `Add development tooling and AI agent configurations to your Frappe project.

Available scaffolds:
  ai          Add CLAUDE.md and AI agent skills for Frappe development
  precommit   Add pre-commit configuration with Frappe semgrep rules
  all         Add all available scaffolds

Examples:
  weg scaffold ai          # Add AI agent configuration
  weg scaffold precommit   # Add pre-commit hooks
  weg scaffold all         # Add everything`,
	Args:              cobra.ExactArgs(1),
	RunE:              runScaffold,
	SilenceUsage:      true,
	ValidArgsFunction: scaffoldCompletion,
}

var scaffoldForce bool

func init() {
	rootCmd.AddCommand(scaffoldCmd)
	scaffoldCmd.Flags().BoolVarP(&scaffoldForce, "force", "f", false, "Overwrite existing files")
}

func scaffoldCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return []string{"ai", "precommit", "all"}, cobra.ShellCompDirectiveNoFileComp
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}

func runScaffold(cmd *cobra.Command, args []string) error {
	scaffoldType := args[0]

	path := "."
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Detect context - should be a weg-managed app
	result, err := config.DetectContext(absPath)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	if result.Context != config.ContextWegApp && result.Context != config.ContextApp {
		return fmt.Errorf("scaffold should be run from a Frappe app directory")
	}

	switch scaffoldType {
	case "ai":
		return scaffoldAI(absPath)
	case "precommit":
		return scaffoldPrecommit(absPath)
	case "all":
		if err := scaffoldAI(absPath); err != nil {
			return err
		}
		return scaffoldPrecommit(absPath)
	default:
		return fmt.Errorf("unknown scaffold type: %s. Use 'ai', 'precommit', or 'all'", scaffoldType)
	}
}

func scaffoldAI(projectPath string) error {
	fmt.Println("Scaffolding AI agent configuration...")

	// Create CLAUDE.md
	claudeMD := `# Frappe App Development Guidelines

This project is a Frappe Framework application. Follow these guidelines strictly.

## Project Structure

` + "```" + `
myapp/
├── myapp/
│   ├── __init__.py
│   ├── hooks.py              # App hooks and overrides
│   ├── modules.txt           # List of modules
│   └── module_name/
│       └── doctype/
│           └── doctype_name/
│               ├── doctype_name.py      # Controller
│               ├── doctype_name.js      # Client script
│               └── doctype_name.json    # Schema
├── pyproject.toml
└── .pre-commit-config.yaml
` + "```" + `

## Critical Rules (ERRORS)

### 1. Multitenancy - NEVER use global variables with DB/cache calls

` + "```python" + `
# BAD - breaks multitenancy
CACHED_USERS = frappe.get_all("User")  # Global scope!

# GOOD - wrap in function
def get_users():
    return frappe.get_all("User")
` + "```" + `

### 2. Always commit changes in controller hooks

` + "```python" + `
# BAD - changes lost after hook completes
def on_submit(self):
    self.status = "Submitted"  # Not saved!

# GOOD - use db_set or save
def on_submit(self):
    self.db_set("status", "Submitted")
` + "```" + `

### 3. Never modify child tables while iterating

` + "```python" + `
# BAD - undefined behavior
for row in self.items:
    if row.qty == 0:
        self.remove(row)

# GOOD - collect then modify
to_remove = [row for row in self.items if row.qty == 0]
for row in to_remove:
    self.remove(row)
` + "```" + `

### 4. Use correct cache methods for multitenancy

` + "```python" + `
# BAD - not multitenant-safe
frappe.cache().set("key", value)

# GOOD - use set_value/get_value
frappe.cache().set_value("key", value)
` + "```" + `

### 5. Never reassign frappe local proxies

` + "```python" + `
# BAD - breaks proxying
frappe.db = some_other_db

# GOOD - use frappe.local
frappe.local.db = some_other_db
` + "```" + `

### 6. Use get_single_value for Single DocTypes

` + "```python" + `
# BAD - not type-safe
value = frappe.db.get_value("System Settings", "System Settings", "field")

# GOOD
value = frappe.db.get_single_value("System Settings", "field")
` + "```" + `

### 7. Specify room for realtime messages

` + "```python" + `
# BAD - broadcasts to ALL users on site
frappe.publish_realtime("event", data)

# GOOD - specify recipient
frappe.publish_realtime("event", data, user=frappe.session.user)
` + "```" + `

### 8. Valid controller hooks only

Valid hooks: ` + "`before_insert`, `after_insert`, `validate`, `before_save`, `on_update`, `before_submit`, `on_submit`, `before_cancel`, `on_cancel`, `on_trash`" + `

### 9. Use keyword argument for orderby

` + "```python" + `
# BAD
query.orderby("creation", "desc")

# GOOD
query.orderby("creation", order=frappe.qb.desc)
` + "```" + `

### 10. No monkey patching - use hooks

` + "```python" + `
# BAD - patching at runtime
from frappe.core.doctype.user import user
user.User.some_method = my_method

# GOOD - use hooks.py doc_events
` + "```" + `

## Warnings

- Use ` + "`frappe.logger()`" + ` instead of ` + "`print()`" + `
- Avoid ` + "`frappe.db.commit()`" + ` - let framework handle transactions
- Prefer list comprehensions over ` + "`map()/filter()`" + `
- Remove ` + "`debug=True`" + ` statements

## Translation Rules

` + "```python" + `
# All user-facing text must be translated
frappe.throw(_("Document not found"))

# Format AFTER translate
_("User {0} not found").format(user)
` + "```" + `

## JavaScript Rules

- Never use ` + "`cur_frm`" + ` (deprecated) - use ` + "`frm`" + `
- Create debounced functions once, not on each call
- Translate button text: ` + "`frm.add_custom_button(__('Process'), fn)`" + `

## The Frappe Way

### Schema Changes - Edit JSON, Then Migrate

` + "```bash" + `
# BAD - direct SQL breaks migrations
mysql -e "ALTER TABLE tabUser ADD COLUMN custom_field VARCHAR(255)"

# GOOD - edit DocType JSON, then migrate
weg migrate
` + "```" + `

### Data Changes - Use Frappe API, Not Raw SQL

` + "```bash" + `
# BAD - bypasses permissions and hooks
mysql -e "UPDATE tabUser SET first_name='John' WHERE name='john@example.com'"

# GOOD - use weg api
weg api call frappe.client.set_value --doctype User --name john@example.com --fieldname first_name --value John

# GOOD - for bulk updates, use filters dict + values dict
weg py "frappe.db.set_value('User', {'user_type': 'Website User'}, {'enabled': 0})"
` + "```" + `

### Reading Data - Use Frappe API

` + "```bash" + `
# BAD
mysql -e "SELECT name FROM tabUser WHERE enabled=1"

# GOOD
weg api call frappe.client.get_list --doctype User --filters '{"enabled": 1}'
` + "```" + `

` + "```python" + `
# BAD - raw SQL for simple list
names = frappe.db.sql("SELECT name FROM tabUser WHERE enabled=1", pluck=True)

# GOOD - use pluck for list[str]
names = frappe.get_all("User", filters={"enabled": 1}, pluck="name")
` + "```" + `

### Creating DocTypes - Edit JSON Files

` + "```bash" + `
# BAD - fragile, not version controlled
curl -X POST localhost:8000/api/resource/DocType -d '...'

# GOOD - create JSON files in doctype directory, then:
weg migrate
` + "```" + `

## Quick Reference

| Instead of | Use |
|------------|-----|
| ` + "`print()`" + ` | ` + "`frappe.logger().info()`" + ` |
| ` + "`frappe.db.commit()`" + ` | Let framework handle it |
| ` + "`cur_frm`" + ` | ` + "`frm`" + ` |
| ` + "`frappe.cache().set()`" + ` | ` + "`frappe.cache().set_value()`" + ` |
| ` + "`frappe.db.get_value(Single, Single, field)`" + ` | ` + "`frappe.db.get_single_value()`" + ` |
| ` + "`map()/filter()`" + ` | List comprehensions |
| Global DB calls | Wrap in functions |
| ` + "`ALTER TABLE`" + ` | Edit JSON + ` + "`weg migrate`" + ` |
| ` + "`mysql -e \"UPDATE...\"`" + ` | ` + "`weg api call frappe.client.set_value`" + ` |
| ` + "`mysql -e \"SELECT...\"`" + ` | ` + "`weg api call frappe.client.get_list`" + ` |
| ` + "`frappe.db.sql(\"SELECT x...\")`" + ` | ` + "`frappe.get_all(pluck=\"x\")`" + ` |
`

	claudePath := filepath.Join(projectPath, "CLAUDE.md")
	if err := writeFileIfNotExists(claudePath, claudeMD); err != nil {
		return err
	}
	fmt.Printf("  Created %s\n", claudePath)

	// Create .claude/commands directory
	commandsDir := filepath.Join(projectPath, ".claude", "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		return fmt.Errorf("failed to create commands directory: %w", err)
	}

	// Create frappe.review skill
	reviewSkill := `---
description: Review Frappe/Python code for antipatterns, best practices, and translation compliance.
---

## User Input

` + "```text" + `
$ARGUMENTS
` + "```" + `

Interpret arguments as file paths or glob patterns to review. If empty, review staged changes.

## Goal

Analyze Frappe Framework code for common antipatterns and produce an actionable report.

## Critical Rules to Check

1. **Multitenancy**: Global variables with ` + "`frappe.db.*`" + `, ` + "`frappe.get_all`" + `, ` + "`frappe.cache`" + ` at module level
2. **Uncommitted changes**: ` + "`self.<attr> = ...`" + ` in hooks without ` + "`db_set()`" + ` or ` + "`save()`" + `
3. **Child table mutation**: Modifying ` + "`self.<table>`" + ` while iterating
4. **Cache methods**: Using ` + "`.set()/.get()`" + ` instead of ` + "`.set_value()/.get_value()`" + `
5. **Proxy reassignment**: ` + "`frappe.db = ...`" + ` instead of ` + "`frappe.local.db = ...`" + `
6. **Single DocType**: Using ` + "`get_value`" + ` instead of ` + "`get_single_value`" + `
7. **Realtime broadcast**: Missing ` + "`user=`" + `, ` + "`doctype=`" + `, or ` + "`room=`" + ` in ` + "`publish_realtime`" + `
8. **Invalid hooks**: Using ` + "`after_save`" + ` (not valid)
9. **Query builder**: Missing ` + "`order=`" + ` keyword in ` + "`orderby()`" + `
10. **Monkey patching**: Modifying imported modules at runtime

## Warnings to Check

- ` + "`print()`" + ` in doctypes (use logger/msgprint)
- ` + "`frappe.db.commit()`" + ` (usually wrong)
- ` + "`map()/filter()`" + ` (use comprehensions)
- ` + "`debug=True`" + ` left in code
- ` + "`cur_frm`" + ` in JavaScript (deprecated)

## The Frappe Way Violations

- **Direct SQL for schema**: ` + "`ALTER TABLE`" + `, ` + "`CREATE TABLE`" + ` - use JSON + migrate
- **Direct SQL for data**: ` + "`UPDATE`" + `, ` + "`INSERT`" + `, ` + "`DELETE`" + ` - use ` + "`weg api`" + ` or ` + "`weg py`" + `
- **Direct SQL for queries**: ` + "`SELECT`" + ` - use ` + "`frappe.get_all`" + ` or ` + "`weg api`" + `
- **curl to create DocTypes**: Use JSON files in doctype directory
- **Direct file writes**: Use ` + "`frappe.get_doc({\"doctype\": \"File\", ...})`" + `

## Translation Checks

- ` + "`frappe.throw()`" + `/` + "`msgprint()`" + ` without ` + "`_()`" + `
- Format before translate (` + "`_('x %s' % y)`" + `)
- Empty translations (` + "`_('')`" + `)
- JavaScript: missing ` + "`__()`" + ` on user text

## Execution

1. Get files to review from args or ` + "`git diff --cached`" + `
2. Read each file and apply rules
3. Output report with file:line references
4. Offer to apply fixes

## Report Format

` + "```markdown" + `
## Frappe Code Review

**Files**: N | **Issues**: X critical, Y warnings, Z translation

| File | Line | Severity | Issue | Fix |
|------|------|----------|-------|-----|
` + "```" + `

## Context

$ARGUMENTS
`

	reviewPath := filepath.Join(commandsDir, "frappe.review.md")
	if err := writeFileIfNotExists(reviewPath, reviewSkill); err != nil {
		return err
	}
	fmt.Printf("  Created %s\n", reviewPath)

	fmt.Println("AI agent configuration complete!")
	return nil
}

func scaffoldPrecommit(projectPath string) error {
	fmt.Println("Scaffolding pre-commit configuration...")

	precommitConfig := `# Pre-commit hooks for Frappe app development
# Install: pip install pre-commit && pre-commit install

repos:
  # Frappe-specific semgrep rules
  - repo: https://github.com/frappe/semgrep-rules
    rev: v0.5.0
    hooks:
      - id: frappe-correctness
        name: Frappe Correctness
      - id: frappe-ux
        name: Frappe UX
      - id: frappe-translate
        name: Frappe Translation

  # Python linting with Ruff
  - repo: https://github.com/astral-sh/ruff-pre-commit
    rev: v0.8.6
    hooks:
      - id: ruff
        args: [--fix, --exit-non-zero-on-fix]
      - id: ruff-format

  # General hooks
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v5.0.0
    hooks:
      - id: trailing-whitespace
        exclude: '\.md$'
      - id: end-of-file-fixer
        exclude: '\.json$'
      - id: check-yaml
      - id: check-json
      - id: check-added-large-files
        args: [--maxkb=500]
      - id: check-merge-conflict
      - id: debug-statements

  # Security scanning
  - repo: https://github.com/PyCQA/bandit
    rev: 1.8.2
    hooks:
      - id: bandit
        args: [-c, pyproject.toml, -r]
        additional_dependencies: ["bandit[toml]"]
        exclude: test_.*\.py$
`

	configPath := filepath.Join(projectPath, ".pre-commit-config.yaml")
	if err := writeFileIfNotExists(configPath, precommitConfig); err != nil {
		return err
	}
	fmt.Printf("  Created %s\n", configPath)

	fmt.Println("\nPre-commit configuration complete!")
	fmt.Println("Run these commands to activate:")
	fmt.Println("  pip install pre-commit")
	fmt.Println("  pre-commit install")
	return nil
}

func writeFileIfNotExists(path, content string) error {
	if !scaffoldForce {
		if _, err := os.Stat(path); err == nil {
			fmt.Printf("  Skipping %s (already exists, use --force to overwrite)\n", path)
			return nil
		}
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	return os.WriteFile(path, []byte(content), 0644)
}
