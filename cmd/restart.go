package cmd

import (
	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart development services",
	Long: `Stop and restart all development services.

Equivalent to running 'weg stop' followed by 'weg start'.

Examples:
  weg restart              # Restart services in background
  weg restart -f           # Restart with TUI (foreground)
  weg restart --no-watch   # Restart without file watcher`,
	RunE: runRestart,
}

func init() {
	rootCmd.AddCommand(restartCmd)
	restartCmd.Flags().BoolVarP(&foreground, "foreground", "f", false, "Run in foreground with TUI")
	restartCmd.Flags().BoolVar(&noWatch, "no-watch", false, "Disable file watcher")
	restartCmd.Flags().BoolVar(&noSync, "no-sync", false, "Skip sync check before starting")
}

func runRestart(cmd *cobra.Command, args []string) error {
	// Stop services first (ignore errors if not running)
	_ = runStop(cmd, args)

	// Start services
	return runStart(cmd, args)
}
