package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/runtime"
	"github.com/gavindsouza/weg/internal/services"
	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart development services",
	Long: `Stop and restart all development services.

Uses fast stop (SIGKILL after 3s timeout) for quick restarts.

Examples:
  weg restart              # Restart services in background
  weg restart -f           # Restart with TUI (foreground)
  weg restart --no-watch   # Restart without file watcher
  weg restart --graceful   # Wait for graceful shutdown before restart`,
	RunE: runRestart,
}

var gracefulRestart bool

func init() {
	rootCmd.AddCommand(restartCmd)
	restartCmd.Flags().BoolVarP(&foreground, "foreground", "f", false, "Run in foreground with TUI")
	restartCmd.Flags().BoolVar(&noWatch, "no-watch", false, "Disable file watcher")
	restartCmd.Flags().BoolVar(&noSync, "no-sync", false, "Skip sync check before starting")
	restartCmd.Flags().BoolVar(&gracefulRestart, "graceful", false, "Wait for graceful shutdown (slower but safer)")
}

func runRestart(cmd *cobra.Command, args []string) error {
	path := "."
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Detect context
	result, err := config.DetectProjectContext(absPath)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	// Determine bench path
	var benchPath string
	switch result.Context {
	case config.ContextWegApp:
		benchPath = result.BenchPath
	case config.ContextWegBench:
		benchPath = result.BenchPath
	default:
		return errors.NotInProject(absPath)
	}

	mgr := services.NewManager(benchPath)

	// Load runtime config to get RunID for precise process killing
	if rtConfig, err := runtime.Load(benchPath); err == nil {
		mgr.RunID = rtConfig.RunID
	}

	// Stop services - use fast stop by default for quick restarts
	PrintInfo("Stopping services...")
	if gracefulRestart {
		_ = mgr.Stop()
	} else {
		_ = mgr.StopFast()
	}

	// Clean up runtime config
	_ = runtime.Remove(benchPath)

	// Start services
	return runStart(cmd, args)
}
