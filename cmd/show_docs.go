package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/api"
	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var showDocsCmd = &cobra.Command{
	Use:   "show-docs [app] [doctype]",
	Short: "Show DocTypes and their structure",
	Long: `Show DocTypes in an app or details of a specific DocType.

Without arguments, lists all custom DocTypes.
With app name, lists DocTypes in that app.
With app and doctype, shows field structure.

Examples:
  weg show-docs                         # List all custom doctypes
  weg show-docs myapp                   # List doctypes in myapp
  weg show-docs myapp MyDocType         # Show fields of MyDocType
  weg show-docs myapp MyDocType --json  # Output as JSON`,
	Args: cobra.MaximumNArgs(2),
	RunE: runShowDocs,
}

var (
	showDocsSite string
	showDocsJSON bool
)

func init() {
	rootCmd.AddCommand(showDocsCmd)
	showDocsCmd.Flags().StringVar(&showDocsSite, "site", "", "Site to query")
	showDocsCmd.Flags().BoolVar(&showDocsJSON, "json", false, "Output as JSON")
}

func runShowDocs(cmd *cobra.Command, args []string) error {
	path := "."
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	result, err := config.DetectContext(absPath)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	var benchPath string
	switch result.Context {
	case config.ContextWegBench:
		benchPath = absPath
	case config.ContextWegApp:
		benchPath = filepath.Join(absPath, ".weg")
	default:
		return fmt.Errorf("not a weg-managed project")
	}

	site := showDocsSite
	if site == "" {
		st, err := state.Load(absPath)
		if err == nil {
			site = st.GetDefaultSite()
		}
		if site == "" {
			currentSitePath := filepath.Join(benchPath, "sites", "currentsite.txt")
			data, _ := os.ReadFile(currentSitePath)
			site = strings.TrimSpace(string(data))
		}
	}

	if site == "" {
		return fmt.Errorf("no site specified and no default site found")
	}

	executor := api.NewExecutor(benchPath, site, "Administrator")

	var script string
	if len(args) == 0 {
		// List all custom doctypes
		script = fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()

try:
    doctypes = frappe.get_all('DocType',
        filters={'custom': 0, 'istable': 0, 'module': ['not in', ['Core', 'Email', 'Desk', 'Printing', 'Website', 'Workflow', 'Custom', 'Integrations', 'Automation', 'Event Streaming', 'Social']]},
        fields=['name', 'module'],
        order_by='module, name'
    )
    print(json.dumps({"success": True, "data": doctypes}))
except Exception as ex:
    print(json.dumps({"success": False, "error": str(ex)}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site)
	} else if len(args) == 1 {
		// List doctypes in specific app/module
		appName := args[0]
		script = fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()

try:
    # Get modules for this app
    app_modules = frappe.get_all('Module Def',
        filters={'app_name': '%s'},
        pluck='name'
    )
    if not app_modules:
        # Try matching by module name directly
        app_modules = ['%s']

    doctypes = frappe.get_all('DocType',
        filters={'module': ['in', app_modules], 'istable': 0},
        fields=['name', 'module', 'is_submittable', 'is_tree'],
        order_by='module, name'
    )
    print(json.dumps({"success": True, "data": doctypes}))
except Exception as ex:
    print(json.dumps({"success": False, "error": str(ex)}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site, appName, strings.Title(strings.ReplaceAll(appName, "_", " ")))
	} else {
		// Show doctype structure
		doctype := args[1]
		script = fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()

try:
    meta = frappe.get_meta('%s')
    fields = []
    for f in meta.fields:
        fields.append({
            'fieldname': f.fieldname,
            'fieldtype': f.fieldtype,
            'label': f.label,
            'reqd': f.reqd,
            'options': f.options if f.fieldtype in ['Link', 'Select', 'Table', 'Table MultiSelect'] else None
        })
    result = {
        'name': meta.name,
        'module': meta.module,
        'is_submittable': meta.is_submittable,
        'is_tree': meta.is_tree,
        'fields': fields
    }
    print(json.dumps({"success": True, "data": result}))
except Exception as ex:
    import traceback
    print(json.dumps({"success": False, "error": str(ex), "traceback": traceback.format_exc()}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site, doctype)
	}

	apiResult, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to get docs: %w", err)
	}

	if !apiResult.Success {
		if apiResult.Traceback != "" {
			fmt.Fprintf(os.Stderr, "%s\n", apiResult.Traceback)
		}
		return fmt.Errorf("failed to get docs: %s", apiResult.Error)
	}

	if showDocsJSON {
		output, _ := json.MarshalIndent(apiResult.Data, "", "  ")
		fmt.Println(string(output))
		return nil
	}

	// Format output based on what we got
	if len(args) < 2 {
		// List of doctypes
		doctypes, ok := apiResult.Data.([]interface{})
		if !ok {
			return fmt.Errorf("unexpected response format")
		}

		if len(doctypes) == 0 {
			fmt.Println("No DocTypes found")
			return nil
		}

		fmt.Printf("%-40s %s\n", "DOCTYPE", "MODULE")
		fmt.Println(strings.Repeat("-", 60))
		for _, dt := range doctypes {
			d := dt.(map[string]interface{})
			name := d["name"].(string)
			module := ""
			if m, ok := d["module"].(string); ok {
				module = m
			}
			fmt.Printf("%-40s %s\n", name, module)
		}
	} else {
		// DocType structure
		data, ok := apiResult.Data.(map[string]interface{})
		if !ok {
			return fmt.Errorf("unexpected response format")
		}

		fmt.Printf("DocType: %s\n", data["name"])
		fmt.Printf("Module: %s\n", data["module"])
		if sub, ok := data["is_submittable"].(float64); ok && sub == 1 {
			fmt.Println("Submittable: Yes")
		}
		if tree, ok := data["is_tree"].(float64); ok && tree == 1 {
			fmt.Println("Tree: Yes")
		}
		fmt.Println()
		fmt.Printf("%-25s %-15s %-8s %s\n", "FIELD", "TYPE", "REQD", "OPTIONS")
		fmt.Println(strings.Repeat("-", 70))

		fields, _ := data["fields"].([]interface{})
		for _, f := range fields {
			field := f.(map[string]interface{})
			fieldname := ""
			if fn, ok := field["fieldname"].(string); ok {
				fieldname = fn
			}
			fieldtype := ""
			if ft, ok := field["fieldtype"].(string); ok {
				fieldtype = ft
			}
			reqd := ""
			if r, ok := field["reqd"].(float64); ok && r == 1 {
				reqd = "*"
			}
			options := ""
			if o, ok := field["options"].(string); ok {
				options = o
			}
			fmt.Printf("%-25s %-15s %-8s %s\n", fieldname, fieldtype, reqd, options)
		}
	}

	return nil
}
