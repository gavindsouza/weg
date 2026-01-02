package config

import (
	"github.com/spf13/cobra"
)

// ConfigCmd is the config command group
var ConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "View and modify weg configuration",
	Long: `View and modify weg configuration settings.

Configuration can be project-local (weg.toml or pyproject.toml) or
global (~/.weg/config.toml).

Examples:
  weg config show                  # Show current config
  weg config get frappe.version    # Get specific value
  weg config set frappe.version 15 # Set value
  weg config list-apps             # List configured apps`,
}

func init() {
	ConfigCmd.AddCommand(showCmd)
	ConfigCmd.AddCommand(getCmd)
	ConfigCmd.AddCommand(setCmd)
	ConfigCmd.AddCommand(listAppsCmd)
}
