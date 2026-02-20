package scheduler

import (
	"fmt"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/api"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/prompt"
	"github.com/spf13/cobra"
)

var (
	purgeSite  string
	purgeForce bool
)

var purgeCmd = &cobra.Command{
	Use:   "purge",
	Short: "Purge failed background jobs",
	Long: `Purge failed background jobs from the queue.

This removes all failed jobs from the RQ Job table. Use with caution
as this cannot be undone.

Examples:
  weg scheduler purge
  weg scheduler purge --force
  weg scheduler purge --site mysite.localhost`,
	RunE: runPurge,
}

func init() {
	SchedulerCmd.AddCommand(purgeCmd)
	purgeCmd.Flags().StringVar(&purgeSite, "site", "", "Site to purge jobs for")
	purgeCmd.Flags().BoolVar(&purgeForce, "force", false, "Skip confirmation")
}

func runPurge(cmd *cobra.Command, args []string) error {
	benchPath, site, err := resolveSite(purgeSite)
	if err != nil {
		return err
	}

	if !purgeForce {
		output.Printf("This will delete all failed jobs for %s", site)
		if !prompt.Confirm("Continue?") {
			output.Print("Cancelled")
			return nil
		}
	}

	executor := api.NewExecutor(benchPath, site, "Administrator")

	script := fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()

try:
    # Count failed jobs first
    count = frappe.db.count('RQ Job', {'status': 'failed'})

    # Delete failed jobs
    frappe.db.delete('RQ Job', {'status': 'failed'})
    frappe.db.commit()

    print(json.dumps({"success": True, "data": {"deleted": count}}))
except Exception as ex:
    frappe.db.rollback()
    print(json.dumps({"success": False, "error": str(ex)}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site)

	result, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to purge jobs: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("failed to purge jobs: %s", result.Error)
	}

	data, ok := result.Data.(map[string]any)
	if ok {
		deleted, _ := data["deleted"].(float64)
		output.Successf("Purged %d failed jobs", int(deleted))
	} else {
		output.Success("Failed jobs purged")
	}

	return nil
}
