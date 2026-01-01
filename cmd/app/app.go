package app

import (
	"github.com/spf13/cobra"
)

// AppCmd is the root command for app management
var AppCmd = &cobra.Command{
	Use:   "app",
	Short: "Manage Frappe apps",
	Long: `Commands for managing Frappe apps in the current project.

Examples:
  weg app list              # List installed apps
  weg app get <url>         # Install an app
  weg app remove <name>     # Remove an app
  weg app switch <branch>   # Switch app branch`,
}

func init() {
	AppCmd.AddCommand(listCmd)
	AppCmd.AddCommand(getCmd)
	AppCmd.AddCommand(removeCmd)
	AppCmd.AddCommand(switchCmd)
	AppCmd.AddCommand(excludeCmd)
	AppCmd.AddCommand(includeCmd)
}
