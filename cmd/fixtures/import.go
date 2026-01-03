package fixtures

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/api"
	"github.com/gavindsouza/weg/internal/completion"
	"github.com/spf13/cobra"
)

var importCmd = &cobra.Command{
	Use:   "import <app>",
	Short: "Import fixtures for an app",
	Long: `Import data fixtures into a site for app development.

Reads fixture files from the app's fixtures directory and loads them.

Examples:
  weg fixtures import myapp
  weg fixtures import myapp --site test.localhost`,
	Args:              cobra.ExactArgs(1),
	RunE:              runImport,
	ValidArgsFunction: completion.CompleteAppNamesForArg(0),
}

var importSite string

func init() {
	FixturesCmd.AddCommand(importCmd)
	importCmd.Flags().StringVarP(&importSite, "site", "s", "", "Site to import into")
}

func runImport(cmd *cobra.Command, args []string) error {
	appName := args[0]

	benchPath, site, err := resolveContext(importSite)
	if err != nil {
		return err
	}

	// Verify app exists
	appPath := filepath.Join(benchPath, "apps", appName)
	if _, err := os.Stat(appPath); os.IsNotExist(err) {
		return fmt.Errorf("app %s not found", appName)
	}

	// Check if fixtures directory exists
	fixturesPath := filepath.Join(appPath, appName, "fixtures")
	if _, err := os.Stat(fixturesPath); os.IsNotExist(err) {
		return fmt.Errorf("no fixtures directory found at %s", fixturesPath)
	}

	fmt.Printf("Importing fixtures for %s into %s...\n", appName, site)

	executor := api.NewExecutor(benchPath, site, "Administrator")

	script := fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()

try:
    from frappe.utils.fixtures import sync_fixtures
    sync_fixtures(app='%s')
    frappe.db.commit()
    print(json.dumps({"success": True, "data": "imported"}))
except Exception as ex:
    frappe.db.rollback()
    import traceback
    print(json.dumps({"success": False, "error": str(ex), "traceback": traceback.format_exc()}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site, appName)

	result, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to import fixtures: %w", err)
	}

	if !result.Success {
		if result.Traceback != "" {
			fmt.Fprintf(os.Stderr, "%s\n", result.Traceback)
		}
		return fmt.Errorf("failed to import fixtures: %s", result.Error)
	}

	// List imported files
	if entries, err := os.ReadDir(fixturesPath); err == nil && len(entries) > 0 {
		fmt.Println("\nImported fixtures:")
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".json") {
				fmt.Printf("  - %s\n", e.Name())
			}
		}
	}

	fmt.Println("\nFixtures imported successfully")
	return nil
}
