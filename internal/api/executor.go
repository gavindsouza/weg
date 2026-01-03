package api

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Executor handles Python code execution for Frappe API calls
type Executor struct {
	BenchPath string
	Site      string
	User      string
	Verbose   bool
}

// Result represents the result of an API call
type Result struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	Traceback string      `json:"traceback,omitempty"`
}

// NewExecutor creates a new API executor
func NewExecutor(benchPath, site, user string) *Executor {
	if user == "" {
		user = "Administrator"
	}
	return &Executor{
		BenchPath: benchPath,
		Site:      site,
		User:      user,
	}
}

// Call executes a frappe.call() with the given method and kwargs
func (e *Executor) Call(method string, kwargs map[string]interface{}) (*Result, error) {
	kwargsJSON, err := json.Marshal(kwargs)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize kwargs: %w", err)
	}

	// Escape for Python string literal
	escapedJSON := strings.ReplaceAll(string(kwargsJSON), `\`, `\\`)
	escapedJSON = strings.ReplaceAll(escapedJSON, `'`, `\'`)

	sitesDir := filepath.Join(e.BenchPath, "sites")
	script := fmt.Sprintf(`import frappe
import json
import sys
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()
frappe.set_user('%s')

try:
    kwargs = json.loads('%s')
    result = frappe.call('%s', **kwargs)
    # Handle non-serializable types
    if hasattr(result, 'as_dict'):
        result = result.as_dict()
    print(json.dumps({"success": True, "data": result}, default=str))
except Exception as ex:
    import traceback
    print(json.dumps({"success": False, "error": str(ex)}), file=sys.stderr)
    sys.exit(1)
finally:
    frappe.destroy()
`, sitesDir, e.Site, e.User, escapedJSON, method)

	return e.executeScript(script)
}

// GetDoc retrieves a single document
func (e *Executor) GetDoc(doctype, name string) (*Result, error) {
	sitesDir := filepath.Join(e.BenchPath, "sites")
	script := fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()
frappe.set_user('%s')

try:
    doc = frappe.get_doc('%s', '%s')
    result = doc.as_dict()
    print(json.dumps({"success": True, "data": result}, default=str))
except Exception as ex:
    print(json.dumps({"success": False, "error": str(ex)}))
finally:
    frappe.destroy()
`, sitesDir, e.Site, e.User, doctype, name)

	return e.executeScript(script)
}

// GetList retrieves a list of documents
func (e *Executor) GetList(doctype string, filters map[string]interface{}, fields []string, limit int, orderBy string) (*Result, error) {
	filtersJSON, _ := json.Marshal(filters)
	fieldsJSON, _ := json.Marshal(fields)
	if len(fields) == 0 {
		fieldsJSON = []byte(`["name"]`)
	}

	escapedFilters := strings.ReplaceAll(string(filtersJSON), `'`, `\'`)
	escapedFields := strings.ReplaceAll(string(fieldsJSON), `'`, `\'`)

	sitesDir := filepath.Join(e.BenchPath, "sites")
	script := fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()
frappe.set_user('%s')

try:
    filters = json.loads('%s') if '%s' != 'null' else None
    fields = json.loads('%s')
    result = frappe.get_list('%s', filters=filters, fields=fields, limit_page_length=%d, order_by='%s' if '%s' else None)
    print(json.dumps({"success": True, "data": result}, default=str))
except Exception as ex:
    print(json.dumps({"success": False, "error": str(ex)}))
finally:
    frappe.destroy()
`, sitesDir, e.Site, e.User, escapedFilters, escapedFilters, escapedFields, doctype, limit, orderBy, orderBy)

	return e.executeScript(script)
}

// Insert creates a new document
func (e *Executor) Insert(doc map[string]interface{}) (*Result, error) {
	docJSON, _ := json.Marshal(doc)
	escapedDoc := strings.ReplaceAll(string(docJSON), `'`, `\'`)

	sitesDir := filepath.Join(e.BenchPath, "sites")
	script := fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()
frappe.set_user('%s')

try:
    doc_data = json.loads('%s')
    doc = frappe.get_doc(doc_data)
    doc.insert()
    frappe.db.commit()
    result = doc.as_dict()
    print(json.dumps({"success": True, "data": result}, default=str))
except Exception as ex:
    frappe.db.rollback()
    print(json.dumps({"success": False, "error": str(ex)}))
finally:
    frappe.destroy()
`, sitesDir, e.Site, e.User, escapedDoc)

	return e.executeScript(script)
}

// Save updates an existing document
func (e *Executor) Save(doc map[string]interface{}) (*Result, error) {
	docJSON, _ := json.Marshal(doc)
	escapedDoc := strings.ReplaceAll(string(docJSON), `'`, `\'`)

	sitesDir := filepath.Join(e.BenchPath, "sites")
	script := fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()
frappe.set_user('%s')

try:
    doc_data = json.loads('%s')
    doc = frappe.get_doc(doc_data['doctype'], doc_data['name'])
    doc.update(doc_data)
    doc.save()
    frappe.db.commit()
    result = doc.as_dict()
    print(json.dumps({"success": True, "data": result}, default=str))
except Exception as ex:
    frappe.db.rollback()
    print(json.dumps({"success": False, "error": str(ex)}))
finally:
    frappe.destroy()
`, sitesDir, e.Site, e.User, escapedDoc)

	return e.executeScript(script)
}

// Delete removes a document
func (e *Executor) Delete(doctype, name string) (*Result, error) {
	sitesDir := filepath.Join(e.BenchPath, "sites")
	script := fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()
frappe.set_user('%s')

try:
    frappe.delete_doc('%s', '%s')
    frappe.db.commit()
    print(json.dumps({"success": True, "data": "deleted"}))
except Exception as ex:
    frappe.db.rollback()
    print(json.dumps({"success": False, "error": str(ex)}))
finally:
    frappe.destroy()
`, sitesDir, e.Site, e.User, doctype, name)

	return e.executeScript(script)
}

// executeScript runs a Python script via devbox
func (e *Executor) executeScript(script string) (*Result, error) {
	venvPython := filepath.Join(e.BenchPath, ".venv", "bin", "python")

	// Write script to temp file
	tmpFile, err := os.CreateTemp("", "weg-api-*.py")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(script); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("failed to write script: %w", err)
	}
	tmpFile.Close()

	// Check if devbox is available
	devboxJSON := filepath.Join(e.BenchPath, "devbox.json")
	useDevbox := false
	if _, err := os.Stat(devboxJSON); err == nil {
		useDevbox = true
	}

	var cmd *exec.Cmd
	if useDevbox {
		// Run via devbox to get proper environment
		cmd = exec.Command("devbox", "run", "-c", e.BenchPath, "--",
			venvPython, tmpFile.Name())
	} else {
		cmd = exec.Command(venvPython, tmpFile.Name())
	}
	cmd.Dir = e.BenchPath

	if e.Verbose {
		fmt.Fprintf(os.Stderr, "Executing in: %s\n", e.BenchPath)
		fmt.Fprintf(os.Stderr, "Script:\n%s\n", script)
	}

	output, err := cmd.Output()
	if err != nil {
		// Try to get stderr for error message
		if exitErr, ok := err.(*exec.ExitError); ok {
			var result Result
			if json.Unmarshal(exitErr.Stderr, &result) == nil {
				return &result, nil
			}
			return nil, fmt.Errorf("execution failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("execution failed: %w", err)
	}

	// Parse the JSON output
	var result Result
	if err := json.Unmarshal(output, &result); err != nil {
		// If not JSON, return raw output as data
		return &Result{
			Success: true,
			Data:    strings.TrimSpace(string(output)),
		}, nil
	}

	return &result, nil
}

// ExecuteRaw executes a raw Python script and returns the result
func (e *Executor) ExecuteRaw(script string) (*Result, error) {
	return e.executeScript(script)
}

// FormatJSON formats data as pretty JSON
func FormatJSON(data interface{}) (string, error) {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
