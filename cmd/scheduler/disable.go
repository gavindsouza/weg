package scheduler

import (
	"fmt"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/api"
	"github.com/spf13/cobra"
)

var disableSite string

var disableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable the scheduler",
	Long: `Disable the background job scheduler.

When disabled, no background jobs will be processed. This is useful
during maintenance or when debugging scheduler-related issues.

Examples:
  weg scheduler disable
  weg scheduler disable --site mysite.localhost`,
	RunE: runDisable,
}

func init() {
	disableCmd.Flags().StringVar(&disableSite, "site", "", "Site to disable scheduler for")
}

func runDisable(cmd *cobra.Command, args []string) error {
	benchPath, site, err := resolveSite(disableSite)
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
    frappe.utils.scheduler.disable_scheduler()
    frappe.db.commit()
    print(json.dumps({"success": True, "data": "scheduler disabled"}))
except Exception as ex:
    frappe.db.rollback()
    print(json.dumps({"success": False, "error": str(ex)}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site)

	result, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to disable scheduler: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("failed to disable scheduler: %s", result.Error)
	}

	fmt.Printf("Scheduler disabled for %s\n", site)
	return nil
}
