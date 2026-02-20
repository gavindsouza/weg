package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/fsutil"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Synchronize environment with configuration",
	Long: `Apply configuration changes to the environment.

Sync compares the current configuration (weg.toml or pyproject.toml)
against the recorded state and applies any differences:

  - Installs new apps
  - Removes deleted apps
  - Updates changed app versions
  - Creates new sites
  - Removes deleted sites

By default, sync will show a preview of changes and ask for confirmation.

Examples:
  weg sync           # Interactive sync with confirmation
  weg sync -y        # Apply changes without confirmation
  weg sync --dry-run # Preview changes without applying`,
	RunE:         runSync,
	SilenceUsage: true, // Don't print usage on runtime errors
}

var (
	dryRun bool
)

func init() {
	rootCmd.AddCommand(syncCmd)
	syncCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without applying")
}

// runSync is the main entry point that detects context and delegates to the appropriate sync function
func runSync(cmd *cobra.Command, args []string) error {
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

	switch result.Context {
	case config.ContextWegApp:
		return syncApp(absPath, result)
	case config.ContextWegBench:
		return syncBench(absPath, result)
	case config.ContextFresh, config.ContextApp, config.ContextBench:
		return errors.NotInProject(absPath)
	default:
		return fmt.Errorf("unknown context")
	}
}

// syncApp synchronizes an app-centric environment
func syncApp(path string, result *config.DetectionResult) error {
	PrintInfo("Syncing app-centric environment...")

	wegDir := filepath.Join(path, ".weg")
	wegTomlPath := filepath.Join(wegDir, "weg.toml")

	// For app-centric projects, try .weg/weg.toml first for sync
	if fsutil.FileExists(wegTomlPath) {
		return syncAppWithWegToml(path, wegDir, wegTomlPath)
	}

	// Fallback: Try pyproject.toml
	return syncAppWithPyproject(path)
}

// syncAppWithWegToml syncs an app-centric project using .weg/weg.toml
func syncAppWithWegToml(path, wegDir, wegTomlPath string) error {
	// Use bench-style sync with .weg/weg.toml
	// ParseWegToml expects directory path, not file path
	benchConfig, err := config.ParseWegToml(wegDir)
	if err != nil {
		return fmt.Errorf("failed to parse .weg/weg.toml: %w", err)
	}

	// Validate config
	if err := config.ValidateBenchConfig(benchConfig); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Load state from project root (state module adds .weg internally)
	st, err := state.Load(path)
	if err != nil {
		st = state.NewState()
	}

	// Compute diff
	diff := state.ComputeDiffFromBenchConfig(benchConfig, st, wegDir)

	// Always sync app services (packages and processes from [tool.weg.services])
	// even if there are no other changes
	if err := syncAppServices(wegDir); err != nil {
		PrintVerbose("Warning: failed to sync app services: %v", err)
	}

	if diff.IsEmpty() {
		PrintInfo("Environment is up to date. Nothing to sync.")
		return nil
	}

	// Show changes
	displayChanges(diff)

	if dryRun {
		PrintInfo("\nDry run - no changes applied.")
		return nil
	}

	// Confirm
	if !confirmSync() {
		PrintInfo("Cancelled.")
		return nil
	}

	// Apply changes using .weg as the bench path
	if err := applyBenchChanges(wegDir, benchConfig, st, diff); err != nil {
		return fmt.Errorf("failed to apply changes: %w", err)
	}

	// Update state
	configHash, _ := state.ComputeConfigHash(wegTomlPath)
	st.UpdateConfigHash(configHash)
	st.Frappe.Version = benchConfig.Frappe.Version
	st.Frappe.Database = benchConfig.Frappe.Database

	if err := st.Save(path); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	PrintInfo("\nSync complete!")
	return nil
}

// syncAppWithPyproject syncs an app-centric project using pyproject.toml
func syncAppWithPyproject(path string) error {
	appConfig, err := config.ParsePyproject(path)
	if err != nil {
		return fmt.Errorf("failed to parse config: no .weg/weg.toml or pyproject.toml found")
	}

	// Validate config
	if err := config.ValidateAppConfig(appConfig); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Load state
	st, err := state.Load(path)
	if err != nil {
		st = state.NewState()
	}

	// Compute diff
	appName := filepath.Base(path)
	diff := state.ComputeDiffFromAppConfig(appConfig, appName, st)

	if diff.IsEmpty() {
		PrintInfo("Environment is up to date. Nothing to sync.")
		return nil
	}

	// Show changes
	displayChanges(diff)

	if dryRun {
		PrintInfo("\nDry run - no changes applied.")
		return nil
	}

	// Confirm
	if !confirmSync() {
		PrintInfo("Cancelled.")
		return nil
	}

	// Apply changes
	if err := applyAppChanges(path, appConfig, st, diff); err != nil {
		return fmt.Errorf("failed to apply changes: %w", err)
	}

	// Update state
	pyprojectPath := filepath.Join(path, "pyproject.toml")
	configHash, _ := state.ComputeConfigHash(pyprojectPath)
	st.UpdateConfigHash(configHash)
	st.Frappe.Version = appConfig.Dev.Frappe
	st.Frappe.Database = appConfig.Dev.Database

	if err := st.Save(path); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	PrintInfo("\nSync complete!")
	return nil
}

// syncBench synchronizes a bench-centric environment
func syncBench(path string, result *config.DetectionResult) error {
	PrintInfo("Syncing bench environment...")

	// Parse config
	wegTomlPath := filepath.Join(path, "weg.toml")
	benchConfig, err := config.ParseWegToml(wegTomlPath)
	if err != nil {
		return fmt.Errorf("failed to parse weg.toml: %w", err)
	}

	// Validate config
	if err := config.ValidateBenchConfig(benchConfig); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Load state
	st, err := state.Load(path)
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	// Compute diff
	diff := state.ComputeDiffFromBenchConfig(benchConfig, st, path)

	if diff.IsEmpty() {
		PrintInfo("Environment is up to date. Nothing to sync.")
		return nil
	}

	// Show changes
	displayChanges(diff)

	if dryRun {
		PrintInfo("\nDry run - no changes applied.")
		return nil
	}

	// Confirm
	if !confirmSync() {
		PrintInfo("Cancelled.")
		return nil
	}

	// Apply changes
	if err := applyBenchChanges(path, benchConfig, st, diff); err != nil {
		return fmt.Errorf("failed to apply changes: %w", err)
	}

	// Update state
	configHash, _ := state.ComputeConfigHash(filepath.Join(path, "weg.toml"))
	st.UpdateConfigHash(configHash)
	st.Frappe.Version = benchConfig.Frappe.Version
	st.Frappe.Database = benchConfig.Frappe.Database

	if err := st.Save(path); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	PrintInfo("\nSync complete!")
	return nil
}
