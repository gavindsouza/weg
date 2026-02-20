package site

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/api"
	"github.com/gavindsouza/weg/internal/completion"
	"github.com/gavindsouza/weg/internal/config"
	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var maintenanceCmd = &cobra.Command{
	Use:   "maintenance",
	Short: "Manage site maintenance mode",
	Long: `Enable or disable maintenance mode for a site.

When maintenance mode is enabled, only Administrator can access the site.
Regular users will see a maintenance message.

Examples:
  weg site maintenance on               # Enable maintenance mode
  weg site maintenance off              # Disable maintenance mode
  weg site maintenance status           # Check current status
  weg site maintenance on --site test   # For specific site`,
}

var maintenanceOnCmd = &cobra.Command{
	Use:               "on [site]",
	Short:             "Enable maintenance mode",
	Args:              cobra.MaximumNArgs(1),
	RunE:              runMaintenanceOn,
	ValidArgsFunction: completion.CompleteSiteNamesForArg(0),
}

var maintenanceOffCmd = &cobra.Command{
	Use:               "off [site]",
	Short:             "Disable maintenance mode",
	Args:              cobra.MaximumNArgs(1),
	RunE:              runMaintenanceOff,
	ValidArgsFunction: completion.CompleteSiteNamesForArg(0),
}

var maintenanceStatusCmd = &cobra.Command{
	Use:               "status [site]",
	Short:             "Check maintenance mode status",
	Args:              cobra.MaximumNArgs(1),
	RunE:              runMaintenanceStatus,
	ValidArgsFunction: completion.CompleteSiteNamesForArg(0),
}

var maintenanceSite string

func init() {
	SiteCmd.AddCommand(maintenanceCmd)
	maintenanceCmd.AddCommand(maintenanceOnCmd)
	maintenanceCmd.AddCommand(maintenanceOffCmd)
	maintenanceCmd.AddCommand(maintenanceStatusCmd)

	maintenanceCmd.PersistentFlags().StringVar(&maintenanceSite, "site", "", "Site to manage")
}

func resolveSiteForMaintenance(args []string) (string, string, error) {
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

	// Determine site
	site := maintenanceSite
	if len(args) > 0 {
		site = args[0]
	}
	if site == "" {
		st, err := state.Load(absPath)
		if err == nil {
			site = st.GetDefaultSite()
		}
	}

	if site == "" {
		return "", "", fmt.Errorf("no site specified and no default site found")
	}

	return benchPath, site, nil
}

func runMaintenanceOn(cmd *cobra.Command, args []string) error {
	benchPath, site, err := resolveSiteForMaintenance(args)
	if err != nil {
		return err
	}

	executor := api.NewExecutor(benchPath, site, "Administrator")

	script := fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()

try:
    frappe.db.set_single_value('System Settings', 'maintenance_mode', 1)
    frappe.db.commit()
    frappe.clear_cache()
    print(json.dumps({"success": True}))
except Exception as ex:
    frappe.db.rollback()
    print(json.dumps({"success": False, "error": str(ex)}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site)

	result, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to enable maintenance mode: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("failed to enable maintenance mode: %s", result.Error)
	}

	fmt.Printf("Maintenance mode enabled for %s\n", site)
	return nil
}

func runMaintenanceOff(cmd *cobra.Command, args []string) error {
	benchPath, site, err := resolveSiteForMaintenance(args)
	if err != nil {
		return err
	}

	executor := api.NewExecutor(benchPath, site, "Administrator")

	script := fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()

try:
    frappe.db.set_single_value('System Settings', 'maintenance_mode', 0)
    frappe.db.commit()
    frappe.clear_cache()
    print(json.dumps({"success": True}))
except Exception as ex:
    frappe.db.rollback()
    print(json.dumps({"success": False, "error": str(ex)}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site)

	result, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to disable maintenance mode: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("failed to disable maintenance mode: %s", result.Error)
	}

	fmt.Printf("Maintenance mode disabled for %s\n", site)
	return nil
}

func runMaintenanceStatus(cmd *cobra.Command, args []string) error {
	benchPath, site, err := resolveSiteForMaintenance(args)
	if err != nil {
		return err
	}

	executor := api.NewExecutor(benchPath, site, "Administrator")

	script := fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()

try:
    mode = frappe.db.get_single_value('System Settings', 'maintenance_mode')
    print(json.dumps({"success": True, "data": {"maintenance_mode": mode}}))
except Exception as ex:
    print(json.dumps({"success": False, "error": str(ex)}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site)

	result, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to check maintenance mode: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("failed to check maintenance mode: %s", result.Error)
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("unexpected response format")
	}

	mode := data["maintenance_mode"]
	modeStr := "off"
	if m, ok := mode.(float64); ok && m == 1 {
		modeStr = "on"
	} else if m, ok := mode.(string); ok && (m == "1" || strings.EqualFold(m, "true")) {
		modeStr = "on"
	}

	fmt.Printf("Maintenance mode for %s: %s\n", site, modeStr)
	return nil
}
