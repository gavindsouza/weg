package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/api"
	"github.com/gavindsouza/weg/internal/completion"
	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/prompt"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var reinstallCmd = &cobra.Command{
	Use:   "reinstall <app>",
	Short: "Reinstall an app (uninstall + install)",
	Long: `Reinstall an app by uninstalling and installing it again.

This is useful during development to reset the app's data
and reload fixtures from scratch.

WARNING: This will delete all data created by the app!

Examples:
  weg app reinstall myapp
  weg app reinstall myapp --site test.localhost
  weg app reinstall myapp --force  # Skip confirmation`,
	Args:              cobra.ExactArgs(1),
	RunE:              runReinstall,
	ValidArgsFunction: completion.CompleteAppNamesForArg(0),
}

var (
	reinstallSite  string
	reinstallForce bool
)

func init() {
	AppCmd.AddCommand(reinstallCmd)
	reinstallCmd.Flags().StringVar(&reinstallSite, "site", "", "Site to reinstall app on")
	reinstallCmd.Flags().BoolVar(&reinstallForce, "force", false, "Skip confirmation")
}

func runReinstall(cmd *cobra.Command, args []string) error {
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
		return fmt.Errorf("app %s not found in apps/", appName)
	}

	// Determine site
	site := reinstallSite
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

	// Confirm
	if !reinstallForce {
		output.Printf("This will reinstall %s on %s", appName, site)
		output.Warning("All data created by this app will be DELETED!")
		if !prompt.Confirm("Continue?") {
			output.Print("Cancelled")
			return nil
		}
	}

	executor := api.NewExecutor(benchPath, site, "Administrator")

	// First, uninstall the app
	output.Infof("Uninstalling %s from %s...", appName, site)

	uninstallScript := fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()

try:
    from frappe.installer import remove_app
    remove_app('%s', dry_run=False, yes=True)
    frappe.db.commit()
    print(json.dumps({"success": True}))
except Exception as ex:
    frappe.db.rollback()
    print(json.dumps({"success": False, "error": str(ex)}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site, appName)

	uninstallResult, err := executor.ExecuteRaw(uninstallScript)
	if err != nil {
		return fmt.Errorf("failed to uninstall app: %w", err)
	}

	if !uninstallResult.Success {
		// If uninstall fails because app isn't installed, continue
		if !strings.Contains(uninstallResult.Error, "not installed") {
			return fmt.Errorf("failed to uninstall app: %s", uninstallResult.Error)
		}
		output.Infof("App %s was not installed, proceeding with install...", appName)
	} else {
		output.Successf("Uninstalled %s", appName)
	}

	// Now, install the app
	output.Infof("Installing %s on %s...", appName, site)

	installScript := fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()

try:
    from frappe.installer import install_app
    install_app('%s')
    frappe.db.commit()
    print(json.dumps({"success": True}))
except Exception as ex:
    frappe.db.rollback()
    print(json.dumps({"success": False, "error": str(ex)}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site, appName)

	installResult, err := executor.ExecuteRaw(installScript)
	if err != nil {
		return fmt.Errorf("failed to install app: %w", err)
	}

	if !installResult.Success {
		return fmt.Errorf("failed to install app: %s", installResult.Error)
	}

	// Clear cache after reinstall
	output.Info("Clearing cache...")

	clearScript := fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()

try:
    frappe.clear_cache()
    print(json.dumps({"success": True}))
except Exception as ex:
    print(json.dumps({"success": False, "error": str(ex)}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site)

	executor.ExecuteRaw(clearScript) // Ignore errors

	output.Successf("Reinstalled %s on %s", appName, site)
	return nil
}
