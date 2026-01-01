package site

import (
	"github.com/spf13/cobra"
)

// SiteCmd is the root command for site management
var SiteCmd = &cobra.Command{
	Use:   "site",
	Short: "Manage Frappe sites",
	Long: `Commands for managing Frappe sites in the current project.

Examples:
  weg site list              # List all sites
  weg site new <name>        # Create a new site
  weg site drop <name>       # Delete a site
  weg site use <name>        # Set default site`,
}

func init() {
	SiteCmd.AddCommand(listCmd)
	SiteCmd.AddCommand(newCmd)
	SiteCmd.AddCommand(dropCmd)
	SiteCmd.AddCommand(useCmd)
	SiteCmd.AddCommand(installCmd)
}
