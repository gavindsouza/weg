package build

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/api"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/spf13/cobra"
)

var clearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear build cache and compiled assets",
	Long: `Remove all compiled assets and build cache.

Use this to force a fresh rebuild of all assets.

Examples:
  weg build clear`,
	RunE: runClear,
}

var clearSite string

func init() {
	BuildCmd.AddCommand(clearCmd)
	clearCmd.Flags().StringVarP(&clearSite, "site", "s", "", "Site to clear for")
}

func runClear(cmd *cobra.Command, args []string) error {
	benchPath, site, err := resolveContext(clearSite)
	if err != nil {
		return err
	}

	output.Print("Clearing build cache and assets...")

	// Clear assets directory bundles
	assetsDir := filepath.Join(benchPath, "sites", "assets")
	cleared := 0

	if entries, err := os.ReadDir(assetsDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				// Clear dist directories
				distDir := filepath.Join(assetsDir, e.Name(), "dist")
				if _, err := os.Stat(distDir); err == nil {
					os.RemoveAll(distDir)
					cleared++
				}
				// Clear node_modules in app assets
				nodeDir := filepath.Join(assetsDir, e.Name(), "node_modules")
				if _, err := os.Stat(nodeDir); err == nil {
					os.RemoveAll(nodeDir)
					cleared++
				}
			}
		}
	}

	// Clear from apps as well
	appsDir := filepath.Join(benchPath, "apps")
	if entries, err := os.ReadDir(appsDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				// Clear node_modules in app source
				nodeDir := filepath.Join(appsDir, e.Name(), "node_modules")
				if _, err := os.Stat(nodeDir); err == nil {
					output.Infof("Removing %s/node_modules...", e.Name())
					os.RemoveAll(nodeDir)
					cleared++
				}
			}
		}
	}

	// Clear Python bytecode cache
	executor := api.NewExecutor(benchPath, site, "Administrator")
	script := fmt.Sprintf(`import frappe
import json
import os
import shutil

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()

try:
    # Clear __pycache__ directories
    apps_path = '%s'
    count = 0
    for root, dirs, files in os.walk(apps_path):
        for d in dirs:
            if d == '__pycache__':
                shutil.rmtree(os.path.join(root, d))
                count += 1
        # Remove .pyc files
        for f in files:
            if f.endswith('.pyc'):
                os.remove(os.path.join(root, f))
                count += 1
    print(json.dumps({"success": True, "data": count}))
except Exception as ex:
    print(json.dumps({"success": False, "error": str(ex)}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site, appsDir)

	result, err := executor.ExecuteRaw(script)
	if err == nil && result.Success {
		if count, ok := result.Data.(float64); ok {
			output.Printf("Cleared %d Python cache files/directories", int(count))
		}
	}

	output.Printf("Cleared %d asset directories", cleared)
	output.Print("Build cache cleared. Run 'weg build assets' to rebuild.")
	return nil
}
