package scheduler

import (
	"github.com/spf13/cobra"
)

// SchedulerCmd is the root command for scheduler management
var SchedulerCmd = &cobra.Command{
	Use:   "scheduler",
	Short: "Manage the background job scheduler",
	Long: `Commands for managing the Frappe background job scheduler.

The scheduler processes background jobs like sending emails,
running scheduled tasks, and other async operations.

Examples:
  weg scheduler status     # Check if scheduler is enabled
  weg scheduler enable     # Enable the scheduler
  weg scheduler disable    # Disable the scheduler`,
}

func init() {
	SchedulerCmd.AddCommand(statusCmd)
	SchedulerCmd.AddCommand(enableCmd)
	SchedulerCmd.AddCommand(disableCmd)
}
