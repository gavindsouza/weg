# Frappe App Development Guidelines

This project is a Frappe Framework application. Follow these guidelines strictly.

## Project Structure

```
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
```

## Critical Rules (ERRORS)

### 1. Multitenancy - NEVER use global variables with DB/cache calls

```python
# BAD - breaks multitenancy
CACHED_USERS = frappe.get_all("User")  # Global scope!

# GOOD - wrap in function
def get_users():
    return frappe.get_all("User")
```

### 2. Always commit changes in controller hooks

```python
# BAD - changes lost after hook completes
def on_submit(self):
    self.status = "Submitted"  # Not saved!

# GOOD - use db_set or save
def on_submit(self):
    self.db_set("status", "Submitted")
    # OR
    self.status = "Submitted"
    self.save()
```

### 3. Never modify child tables while iterating

```python
# BAD - undefined behavior
for row in self.items:
    if row.qty == 0:
        self.remove(row)

# GOOD - collect then modify
to_remove = [row for row in self.items if row.qty == 0]
for row in to_remove:
    self.remove(row)
```

### 4. Use correct cache methods for multitenancy

```python
# BAD - not multitenant-safe
frappe.cache().set("key", value)
frappe.cache().get("key")

# GOOD - use set_value/get_value
frappe.cache().set_value("key", value)
frappe.cache().get_value("key")
```

### 5. Never reassign frappe local proxies

```python
# BAD - breaks proxying
frappe.db = some_other_db
frappe.session = custom_session

# GOOD - use frappe.local
frappe.local.db = some_other_db
```

### 6. Use get_single_value for Single DocTypes

```python
# BAD - not type-safe
value = frappe.db.get_value("System Settings", "System Settings", "field")
frappe.db.set_value("System Settings", None, "field", value)

# GOOD
value = frappe.db.get_single_value("System Settings", "field")
frappe.db.set_single_value("System Settings", "field", value)
```

### 7. Specify room for realtime messages

```python
# BAD - broadcasts to ALL users on site
frappe.publish_realtime("event", data)

# GOOD - specify recipient
frappe.publish_realtime("event", data, user=frappe.session.user)
frappe.publish_realtime("event", data, doctype="Sales Order", docname=self.name)
```

### 8. Valid controller hooks only

```python
# BAD - after_save is NOT a valid hook
def after_save(self):
    pass

# GOOD - valid hooks are:
# before_insert, after_insert, validate, before_save, on_update,
# before_submit, on_submit, before_cancel, on_cancel, on_trash
```

### 9. Use keyword argument for orderby

```python
# BAD
query.orderby("creation", "desc")
query.orderby("creation", frappe.qb.desc)

# GOOD
query.orderby("creation", order=frappe.qb.desc)
```

### 10. No monkey patching - use hooks

```python
# BAD - patching at runtime
from frappe.core.doctype.user import user
user.User.some_method = my_method

# GOOD - use hooks.py
doc_events = {
    "User": {
        "validate": "myapp.overrides.user.validate"
    }
}
```

### 11. Never use eval() - use safe_eval()

```python
# BAD - code injection risk
result = eval(user_input)

# GOOD - sandboxed evaluation
from frappe.utils.safe_exec import safe_eval
result = safe_eval(user_input)
```

### 12. Don't duplicate dict keys

```python
# BAD - second value overwrites first, likely a bug
data = {"name": "John", "age": 30, "name": "Jane"}

# GOOD
data = {"name": "Jane", "age": 30}
```

## Warnings

### Use msgprint/logger instead of print

```python
# BAD
print("Debug:", value)

# GOOD
frappe.logger().info(f"Debug: {value}")
frappe.msgprint(f"Value: {value}")  # For user-facing messages
```

### Don't use `args` as sole function argument

```python
# BAD - reduces readability, allows ill-specified arguments
def process(args):
    name = args.get("name")

# GOOD - explicit parameters
def process(name, value, doctype=None):
    pass
```

### Use frappe.get_doc() directly

```python
# BAD - unnecessary dict()
doc = frappe.get_doc(dict(doctype="User", email="test@example.com"))

# GOOD - pass kwargs directly
doc = frappe.get_doc(doctype="User", email="test@example.com")
```

### Avoid manual commits

```python
# BAD - usually indicates misunderstanding of tx model
frappe.db.commit()

# GOOD - let the framework handle commits
# Only use manual commit in background jobs or specific cases
# If needed, add a comment explaining why
```

### Avoid map/filter - use comprehensions

```python
# BAD - mixing paradigms
names = list(map(lambda x: x.name, docs))

# GOOD - Pythonic
names = [d.name for d in docs]
```

### Remove debug statements

```python
# BAD - forgot to remove
result = frappe.get_all("User", debug=True)

# GOOD
result = frappe.get_all("User")
```

## Translation Rules

### All user-facing text must be translated

```python
# BAD
frappe.throw("Document not found")
frappe.msgprint("Success!")

# GOOD
frappe.throw(_("Document not found"))
frappe.msgprint(_("Success!"))
```

### Use positional formatters, format AFTER translate

```python
# BAD
frappe.throw(_("User %s not found" % user))
frappe.throw(_("User {} not found".format(user)))

# GOOD
frappe.throw(_("User {0} not found").format(user))
```

### Never translate empty or variable-only strings

```python
# BAD
_("")
_("{0}")

# GOOD - include context
_("No results found")
_("Created {0} records").format(count)
```

### No leading/trailing whitespace in translations

```python
# BAD - whitespace breaks translation matching
_("  Hello World  ")
_("\tIndented")

# GOOD
_("Hello World")
```

### Don't split or concatenate translations

```python
# BAD - translators can't see full context
_("Hello") + _(" World")
_("This is a very long " + "string that spans lines")

# GOOD - keep as single string
_("Hello World")
_("This is a very long string that spans lines")
```

### Translate dynamic labels from meta

```python
# BAD - label is not translated
msg = _("Value required for {0}").format(self.meta.get_label("field"))

# GOOD - translate the label too
msg = _("Value required for {0}").format(_(self.meta.get_label("field")))
```

### Report filter options - use label/value objects

```javascript
// BAD - translated values in business logic are problematic
filters: [{
    fieldname: "status",
    options: [__("Open"), __("Closed")]
}]

// GOOD - separate label from value
filters: [{
    fieldname: "status",
    options: [
        {label: __("Open"), value: "Open"},
        {label: __("Closed"), value: "Closed"}
    ]
}]
```

### JavaScript translation

```javascript
// BAD
frappe.msgprint("Error occurred");
frappe.throw(`Error: ${msg}`);  // Template literal not allowed
frappe.show_alert("Success!");

// GOOD
frappe.msgprint(__("Error occurred"));
frappe.throw(__("Error: {0}", [msg]));
frappe.show_alert(__("Success!"));
```

### Button text must be translated

```javascript
// BAD
frm.add_custom_button("Process", callback);

// GOOD
frm.add_custom_button(__("Process"), callback);
```

## JavaScript Rules

### Never use cur_frm

```javascript
// BAD - deprecated, buggy
cur_frm.set_value("field", value);

// GOOD - use frm from context
frm.set_value("field", value);
```

### Use debounce correctly

```javascript
// BAD - creates new debounced function each time
button.onclick = () => frappe.utils.debounce(handler, 300)();

// GOOD - create once, call many
const debouncedHandler = frappe.utils.debounce(handler, 300);
button.onclick = debouncedHandler;
```

### Use vanilla JS instead of frappe wrappers

```javascript
// BAD - unnecessary wrapper
if (in_list(myList, item)) { ... }

// GOOD - vanilla JS
if (myList.includes(item)) { ... }
```

## The Frappe Way

These patterns ensure changes are tracked, version-controlled, and properly migrated.

### Schema Changes - Edit JSON, Then Migrate

```bash
# BAD - direct SQL breaks migrations, not version-controlled
mysql -e "ALTER TABLE tabUser ADD COLUMN custom_field VARCHAR(255)"

# GOOD - edit the DocType JSON, then migrate
# 1. Edit myapp/doctype/my_doctype/my_doctype.json
# 2. Add field to "fields" array
# 3. Run migrate
weg migrate
```

For Custom Fields on standard DocTypes:
```bash
# Create a Custom Field via fixtures or the UI
# Then export to keep in version control:
weg export-fixtures
```

### Data Changes - Use Frappe API, Not Raw SQL

```bash
# BAD - bypasses permissions, hooks, and validation
mysql -e "UPDATE tabUser SET first_name='John' WHERE name='john@example.com'"

# GOOD - use weg api (respects permissions and triggers hooks)
weg api call frappe.client.set_value \
  --doctype User \
  --name john@example.com \
  --fieldname first_name \
  --value John

# GOOD - for bulk updates, use filters dict + values dict
weg py "frappe.db.set_value('User', {'user_type': 'Website User'}, {'enabled': 0})"
```

### Reading Data - Use Frappe API

```bash
# BAD - requires knowing table structure, no permission checks
mysql -e "SELECT name, first_name FROM tabUser WHERE enabled=1"

# GOOD - respects permissions, returns proper types
weg api call frappe.client.get_list \
  --doctype User \
  --filters '{"enabled": 1}' \
  --fields '["name", "first_name"]'

# GOOD - for complex queries
weg py "print(frappe.get_all('User', filters={'enabled': 1}, fields=['name', 'first_name']))"
```

```python
# BAD - raw SQL for simple list
names = frappe.db.sql("SELECT name FROM tabUser WHERE enabled=1", pluck=True)

# GOOD - use pluck parameter for list[str]
names = frappe.get_all("User", filters={"enabled": 1}, pluck="name")
# Returns: ["user1@example.com", "user2@example.com", ...]
```

### Creating DocTypes - Edit JSON, Not API

```bash
# BAD - fragile, hard to version control
curl -X POST localhost:8000/api/resource/DocType -d '{"doctype": "DocType", ...}'

# GOOD - create the directory structure and JSON files
mkdir -p myapp/myapp/module/doctype/my_doctype
# Create my_doctype.json with proper schema
# Create my_doctype.py for controller
# Then sync:
weg migrate
```

### Fixtures for Master Data

```python
# In hooks.py - for data that should exist in every installation
fixtures = [
    {"dt": "Custom Field", "filters": [["module", "=", "My App"]]},
    {"dt": "Property Setter", "filters": [["module", "=", "My App"]]},
    {"dt": "My Master DocType", "filters": [["is_standard", "=", 1]]},
]
```

```bash
# Export fixtures to JSON files
weg export-fixtures

# Import fixtures (runs during migrate)
weg migrate
```

### Background Jobs - Use Frappe's Job System

```python
# BAD - blocks the request
def long_running_task():
    for doc in frappe.get_all("BigDocType"):
        process(doc)  # Takes forever

# GOOD - enqueue for background processing
frappe.enqueue(
    "myapp.tasks.process_all",
    queue="long",
    timeout=3600
)
```

### File Operations - Use Frappe's File API

```python
# BAD - direct filesystem access
with open("/home/frappe/files/doc.pdf", "wb") as f:
    f.write(content)

# GOOD - use Frappe's File doctype
file_doc = frappe.get_doc({
    "doctype": "File",
    "file_name": "doc.pdf",
    "content": content,
    "attached_to_doctype": "Sales Invoice",
    "attached_to_name": invoice.name
})
file_doc.insert()
```

## Testing

Run tests with:
```bash
weg test --app myapp
# or specific module
weg test --app myapp --module module_name
```

## Quick Reference

| Instead of | Use |
|------------|-----|
| `print()` | `frappe.logger().info()` |
| `frappe.db.commit()` | Let framework handle it |
| `cur_frm` | `frm` |
| `frappe.cache().set()` | `frappe.cache().set_value()` |
| `frappe.db.get_value(Single, Single, field)` | `frappe.db.get_single_value()` |
| `map()/filter()` | List comprehensions |
| Global DB calls | Wrap in functions |
| `ALTER TABLE` | Edit JSON + `weg migrate` |
| `mysql -e "UPDATE..."` | `weg api call frappe.client.set_value` |
| `mysql -e "SELECT..."` | `weg api call frappe.client.get_list` |
| `frappe.db.sql("SELECT x FROM tabY")` | `frappe.get_all("Y", pluck="x")` |
| Direct file writes | `frappe.get_doc({"doctype": "File", ...})` |
| Inline long tasks | `frappe.enqueue()` |
| `eval()` | `frappe.utils.safe_exec.safe_eval()` |
| `in_list(arr, x)` | `arr.includes(x)` |
| `frappe.get_doc(dict(...))` | `frappe.get_doc(...)` |
| `def process(args):` | `def process(name, value):` |

## Pre-commit

This project uses pre-commit hooks. Install with:
```bash
pip install pre-commit
pre-commit install
```

Hooks run automatically on commit. To run manually:
```bash
pre-commit run --all-files
```
