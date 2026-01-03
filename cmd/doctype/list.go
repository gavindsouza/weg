package doctype

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/api"
	"github.com/gavindsouza/weg/internal/completion"
	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list [app]",
	Short: "List DocTypes",
	Long: `List DocTypes, optionally filtered by app.

Examples:
  weg doctype list              # List all custom doctypes
  weg doctype list myapp        # List doctypes in myapp
  weg doctype list --all        # Include core doctypes`,
	Args:              cobra.MaximumNArgs(1),
	RunE:              runList,
	ValidArgsFunction: completion.CompleteAppNamesForArg(0),
}

var (
	listSite string
	listAll  bool
)

func init() {
	DoctypeCmd.AddCommand(listCmd)
	listCmd.Flags().StringVarP(&listSite, "site", "s", "", "Site to query")
	listCmd.Flags().BoolVar(&listAll, "all", false, "Include core doctypes")
}

func runList(cmd *cobra.Command, args []string) error {
	benchPath, site, err := resolveContext(listSite)
	if err != nil {
		return err
	}

	executor := api.NewExecutor(benchPath, site, "Administrator")

	var script string
	if len(args) == 0 {
		// List all custom doctypes
		coreFilter := ""
		if !listAll {
			coreFilter = ", 'module': ['not in', ['Core', 'Email', 'Desk', 'Printing', 'Website', 'Workflow', 'Custom', 'Integrations', 'Automation', 'Event Streaming', 'Social']]"
		}
		script = fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()

try:
    doctypes = frappe.get_all('DocType',
        filters={'istable': 0%s},
        fields=['name', 'module'],
        order_by='module, name'
    )
    print(json.dumps({"success": True, "data": doctypes}))
except Exception as ex:
    print(json.dumps({"success": False, "error": str(ex)}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site, coreFilter)
	} else {
		// List doctypes in specific app
		appName := args[0]
		script = fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()

try:
    app_modules = frappe.get_all('Module Def',
        filters={'app_name': '%s'},
        pluck='name'
    )
    if not app_modules:
        app_modules = ['%s']

    doctypes = frappe.get_all('DocType',
        filters={'module': ['in', app_modules], 'istable': 0},
        fields=['name', 'module', 'is_submittable'],
        order_by='module, name'
    )
    print(json.dumps({"success": True, "data": doctypes}))
except Exception as ex:
    print(json.dumps({"success": False, "error": str(ex)}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site, appName, strings.Title(strings.ReplaceAll(appName, "_", " ")))
	}

	result, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to list doctypes: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("failed to list doctypes: %s", result.Error)
	}

	doctypes, ok := result.Data.([]interface{})
	if !ok || len(doctypes) == 0 {
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

	return nil
}

func resolveContext(siteName string) (string, string, error) {
	path := "."
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", "", fmt.Errorf("invalid path: %w", err)
	}

	result, err := config.DetectContext(absPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to detect context: %w", err)
	}

	var benchPath string
	switch result.Context {
	case config.ContextWegBench:
		benchPath = absPath
	case config.ContextWegApp:
		benchPath = filepath.Join(absPath, ".weg")
	default:
		return "", "", fmt.Errorf("not a weg-managed project")
	}

	site := siteName
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
		return "", "", fmt.Errorf("no site specified and no default site found")
	}

	return benchPath, site, nil
}
