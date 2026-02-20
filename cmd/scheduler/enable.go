package scheduler

import (
	"fmt"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/api"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/spf13/cobra"
)

var enableSite string

var enableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Enable the scheduler",
	Long: `Enable the background job scheduler.

When enabled, the scheduler will process background jobs like
sending emails, running scheduled tasks, and other async operations.

Examples:
  weg scheduler enable
  weg scheduler enable --site mysite.localhost`,
	RunE: runEnable,
}

func init() {
	enableCmd.Flags().StringVar(&enableSite, "site", "", "Site to enable scheduler for")
}

func runEnable(cmd *cobra.Command, args []string) error {
	benchPath, site, err := resolveSite(enableSite)
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
    frappe.utils.scheduler.enable_scheduler()
    frappe.db.commit()
    print(json.dumps({"success": True, "data": "scheduler enabled"}))
except Exception as ex:
    frappe.db.rollback()
    print(json.dumps({"success": False, "error": str(ex)}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site)

	result, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to enable scheduler: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("failed to enable scheduler: %s", result.Error)
	}

	output.Printf("Scheduler enabled for %s", site)
	return nil
}
