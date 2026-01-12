package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/runtime"
	"github.com/gavindsouza/weg/internal/services"
	"github.com/spf13/cobra"
)

var forceStop bool

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop development services",
	Long: `Stop all running development services.

Examples:
  weg stop           # Stop all services (graceful)
  weg stop --force   # Force stop with SIGKILL (faster)`,
	RunE: runStop,
}

func init() {
	rootCmd.AddCommand(stopCmd)
	stopCmd.Flags().BoolVarP(&forceStop, "force", "F", false, "Force stop with SIGKILL (faster, but may interrupt jobs)")
}

func runStop(cmd *cobra.Command, args []string) error {
	path := "."
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Detect context
	result, err := config.DetectContext(absPath)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	// Determine bench path
	var benchPath string
	switch result.Context {
	case config.ContextWegApp:
		benchPath = filepath.Join(absPath, ".weg")
	case config.ContextWegBench:
		benchPath = absPath
	default:
		return fmt.Errorf("not a weg-managed project")
	}

	mgr := services.NewManager(benchPath)

	// Load runtime config to get RunID for precise process killing
	if rtConfig, err := runtime.Load(benchPath); err == nil {
		mgr.RunID = rtConfig.RunID
	}

	PrintInfo("Stopping services...")
	var stopErr error
	if forceStop {
		stopErr = mgr.StopFast()
	} else {
		stopErr = mgr.Stop()
	}
	if stopErr != nil {
		return stopErr
	}

	// Clean up runtime config
	if err := runtime.Remove(benchPath); err != nil {
		PrintVerbose("Warning: failed to remove runtime config: %v", err)
	}

	PrintInfo("Services stopped.")
	return nil
}
