package cloud

import (
	"github.com/spf13/cobra"
)

var CloudCmd = &cobra.Command{
	Use:   "cloud",
	Short: "Frappe Cloud operations",
	Long: `Interact with Frappe Cloud for deployment and management.

Authenticate with Frappe Cloud and deploy your apps directly
from the command line.

Examples:
  weg cloud login              # Authenticate with Frappe Cloud
  weg cloud sites              # List your sites
  weg cloud deploy             # Deploy to a site
  weg cloud logs               # View site logs
  weg cloud status             # Check deployment status`,
}

func init() {
	CloudCmd.AddCommand(loginCmd)
	CloudCmd.AddCommand(logoutCmd)
	CloudCmd.AddCommand(sitesCmd)
	CloudCmd.AddCommand(benchesCmd)
	CloudCmd.AddCommand(deployCmd)
	CloudCmd.AddCommand(statusCmd)
	CloudCmd.AddCommand(cloudLogsCmd)
}
