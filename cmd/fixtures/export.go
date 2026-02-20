package fixtures

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/api"
	"github.com/gavindsouza/weg/internal/completion"
	"github.com/gavindsouza/weg/internal/config"
	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var exportCmd = &cobra.Command{
	Use:   "export <app>",
	Short: "Export fixtures for an app",
	Long: `Export data fixtures from a site for app development.

Fixtures are saved to the app's fixtures directory.

Examples:
  weg fixtures export myapp
  weg fixtures export myapp --doctype Role
  weg fixtures export myapp --site test.localhost`,
	Args:              cobra.ExactArgs(1),
	RunE:              runExport,
	ValidArgsFunction: completion.CompleteAppNamesForArg(0),
}

var (
	exportSite    string
	exportDoctype string
)

func init() {
	FixturesCmd.AddCommand(exportCmd)
	exportCmd.Flags().StringVarP(&exportSite, "site", "s", "", "Site to export from")
	exportCmd.Flags().StringVar(&exportDoctype, "doctype", "", "Specific doctype to export")
}

func runExport(cmd *cobra.Command, args []string) error {
	appName := args[0]

	benchPath, site, err := resolveContext(exportSite)
	if err != nil {
		return err
	}

	// Verify app exists
	appPath := filepath.Join(benchPath, "apps", appName)
	if _, err := os.Stat(appPath); os.IsNotExist(err) {
		return fmt.Errorf("app %s not found", appName)
	}

	output.Infof("Exporting fixtures for %s from %s...\n", appName, site)

	executor := api.NewExecutor(benchPath, site, "Administrator")

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
    from frappe.utils.fixtures import export_fixtures
    export_fixtures(app='%s')
    print(json.dumps({"success": True, "data": "exported"}))
except Exception as ex:
    import traceback
    print(json.dumps({"success": False, "error": str(ex), "traceback": traceback.format_exc()}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site, appName, doctypeFilter, appName)

	result, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to export fixtures: %w", err)
	}

	if !result.Success {
		if result.Traceback != "" {
			fmt.Fprintf(os.Stderr, "%s\n", result.Traceback)
		}
		return fmt.Errorf("failed to export fixtures: %s", result.Error)
	}

	// Show where fixtures were saved
	fixturesPath := filepath.Join(appPath, appName, "fixtures")
	fmt.Printf("Fixtures exported to: %s\n", fixturesPath)

	// List exported files
	if entries, err := os.ReadDir(fixturesPath); err == nil && len(entries) > 0 {
		fmt.Println("\nExported files:")
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".json") {
				fmt.Printf("  - %s\n", e.Name())
			}
		}
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
		return "", "", wegerrors.NotInProject(absPath)
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
