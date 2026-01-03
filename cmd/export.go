package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/api"
	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var exportCmd = &cobra.Command{
	Use:   "export-fixtures",
	Short: "Export data fixtures for an app",
	Long: `Export data fixtures from a site for use in app development.

Fixtures are JSON files that can be loaded during app installation
to provide default data like roles, workflows, print formats, etc.

The exported files are saved to the app's fixtures directory.

Examples:
  weg export-fixtures myapp                    # Export all fixtures for myapp
  weg export-fixtures myapp --doctype Role     # Export only Role documents
  weg export-fixtures myapp --site test        # From specific site`,
	Args: cobra.ExactArgs(1),
	RunE: runExportFixtures,
}

var (
	exportSite    string
	exportDoctype string
)

func init() {
	rootCmd.AddCommand(exportCmd)
	exportCmd.Flags().StringVar(&exportSite, "site", "", "Site to export from")
	exportCmd.Flags().StringVar(&exportDoctype, "doctype", "", "Specific doctype to export")
}

func runExportFixtures(cmd *cobra.Command, args []string) error {
	appName := args[0]

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

	// Verify app exists
	appPath := filepath.Join(benchPath, "apps", appName)
	if _, err := os.Stat(appPath); os.IsNotExist(err) {
		return fmt.Errorf("app %s not found", appName)
	}

	// Determine site
	site := exportSite
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

	PrintInfo("Exporting fixtures for %s from %s...", appName, site)

	executor := api.NewExecutor(benchPath, site, "Administrator")

	// Build the export script
	doctypeFilter := ""
	if exportDoctype != "" {
		doctypeFilter = fmt.Sprintf(", doctype='%s'", exportDoctype)
	}

	script := fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()

try:
    from frappe.core.doctype.data_import.data_import import export_fixture
    result = export_fixture('%s'%s)
    print(json.dumps({"success": True, "data": result or "exported"}))
except AttributeError:
    # Fallback for older Frappe versions
    from frappe.utils.fixtures import export_fixtures
    export_fixtures(app='%s')
    print(json.dumps({"success": True, "data": "exported"}))
except Exception as ex:
    print(json.dumps({"success": False, "error": str(ex)}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site, appName, doctypeFilter, appName)

	apiResult, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to export fixtures: %w", err)
	}

	if !apiResult.Success {
		return fmt.Errorf("failed to export fixtures: %s", apiResult.Error)
	}

	// Show where fixtures were saved
	fixturesPath := filepath.Join(appPath, appName, "fixtures")
	PrintInfo("Fixtures exported to: %s", fixturesPath)

	// List exported files
	if entries, err := os.ReadDir(fixturesPath); err == nil && len(entries) > 0 {
		PrintInfo("")
		PrintInfo("Exported files:")
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".json") {
				PrintInfo("  - %s", e.Name())
			}
		}
	}

	return nil
}
