package cmd

import (
	"github.com/spf13/cobra"
)

var consoleCmd = &cobra.Command{
	Use:   "console",
	Short: "Open a Python console with Frappe context",
	Long: `Open an interactive Python console with Frappe loaded.

Examples:
  weg console                    # Use default site
  weg console --site mysite      # Use specific site`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmdArgs := []string{"console"}
		if consoleSite != "" {
			cmdArgs = []string{"frappe", "--site", consoleSite, "console"}
		}
		return RunBench(cmdArgs)
	},
	SilenceUsage: true,
}

var mariadbCmd = &cobra.Command{
	Use:     "mariadb",
	Aliases: []string{"mysql"},
	Short:   "Open a MariaDB/MySQL shell",
	Long: `Open an interactive database shell for the selected site.

Examples:
  weg mariadb                    # Use default site
  weg mariadb --site mysite      # Use specific site`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmdArgs := []string{"db-console"}
		if mariadbSite != "" {
			cmdArgs = []string{"frappe", "--site", mariadbSite, "db-console"}
		}
		return RunBench(cmdArgs)
	},
	SilenceUsage: true,
}

var (
	consoleSite string
	mariadbSite string
)

func init() {
	rootCmd.AddCommand(consoleCmd)
	rootCmd.AddCommand(mariadbCmd)

	consoleCmd.Flags().StringVar(&consoleSite, "site", "", "Site to use (default: current site)")
	mariadbCmd.Flags().StringVar(&mariadbSite, "site", "", "Site to use (default: current site)")
}
