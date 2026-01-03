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

## Warnings

### Use msgprint/logger instead of print

```python
# BAD
print("Debug:", value)

# GOOD
frappe.logger().info(f"Debug: {value}")
frappe.msgprint(f"Value: {value}")  # For user-facing messages
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

### JavaScript translation

```javascript
// BAD
frappe.msgprint("Error occurred");
frappe.throw(`Error: ${msg}`);  // Template literal

// GOOD
frappe.msgprint(__("Error occurred"));
frappe.throw(__("Error: {0}", [msg]));
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
