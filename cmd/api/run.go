package api

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	internalapi "github.com/gavindsouza/weg/internal/api"
	"github.com/spf13/cobra"
)

var runArgs string

var runCmd = &cobra.Command{
	Use:   "run <doctype>/<name> <method> [key=value...]",
	Short: "Run a method on a document",
	Long: `Execute a whitelisted method on a specific document.

This calls doc.method(**kwargs) on the specified document,
useful for methods like submit(), cancel(), or custom methods.

Examples:
  weg api run "Sales Invoice/INV-001" submit
  weg api run "Sales Invoice/INV-001" cancel
  weg api run User/Administrator get_fullname
  weg api run ToDo/TODO-001 custom_action arg1=value1 arg2=value2
  weg api run Issue/ISS-001 split --args '{"new_subject":"Part 2"}'`,
	Args:         cobra.MinimumNArgs(2),
	RunE:         runRunCmd,
	SilenceUsage: true,
}

func init() {
	runCmd.Flags().StringVar(&runArgs, "args", "", "JSON object of arguments")
	ApiCmd.AddCommand(runCmd)
}

func runRunCmd(cmd *cobra.Command, args []string) error {
	benchPath, site, err := detectBenchAndSite()
	if err != nil {
		return err
	}

	// Parse doctype/name
	docRef := args[0]
	if !strings.Contains(docRef, "/") {
		return fmt.Errorf("expected format: <doctype>/<name>")
	}

	parts := strings.SplitN(docRef, "/", 2)
	doctype := parts[0]
	name := parts[1]
	method := args[1]

	// Parse kwargs
	kwargs := make(map[string]interface{})

	if runArgs != "" {
		if err := json.Unmarshal([]byte(runArgs), &kwargs); err != nil {
			return fmt.Errorf("invalid --args JSON: %w", err)
		}
	}

	// Parse key=value arguments
	for _, arg := range args[2:] {
		kv := strings.SplitN(arg, "=", 2)
		if len(kv) != 2 {
			return fmt.Errorf("invalid argument format: %s (expected key=value)", arg)
		}
		key := kv[0]
		value := kv[1]

		// Try to parse as JSON for complex values
		var jsonValue interface{}
		if err := json.Unmarshal([]byte(value), &jsonValue); err == nil {
			kwargs[key] = jsonValue
		} else {
			kwargs[key] = value
		}
	}

	result, err := runDocMethod(benchPath, site, apiUser, doctype, name, method, kwargs)
	if err != nil {
		return err
	}

	return printResult(result)
}

func runDocMethod(benchPath, site, user, doctype, name, method string, kwargs map[string]interface{}) (*internalapi.Result, error) {
	kwargsJSON, _ := json.Marshal(kwargs)
	escapedKwargs := strings.ReplaceAll(string(kwargsJSON), `\`, `\\`)
	escapedKwargs = strings.ReplaceAll(escapedKwargs, `'`, `\'`)

	sitesDir := filepath.Join(benchPath, "sites")
	script := fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()
frappe.set_user('%s')

try:
    kwargs = json.loads('%s')
    doc = frappe.get_doc('%s', '%s')
    method = getattr(doc, '%s', None)
    if method is None:
        raise AttributeError("Method '%s' not found on %s")
    if not callable(method):
        raise TypeError("'%s' is not callable")

    result = method(**kwargs)

    # Handle different return types
    if hasattr(result, 'as_dict'):
        result = result.as_dict()
    elif result is None:
        result = {"status": "ok"}

    frappe.db.commit()
    print(json.dumps({"success": True, "data": result}, default=str))
except Exception as ex:
    frappe.db.rollback()
    print(json.dumps({"success": False, "error": str(ex)}))
finally:
    frappe.destroy()
`, sitesDir, site, user, escapedKwargs, doctype, name, method, method, doctype, method)

	executor := internalapi.NewExecutor(benchPath, site, user)
	return executor.ExecuteRaw(script)
}
