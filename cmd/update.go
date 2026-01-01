package cmd

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/gavindsouza/weg/internal/apps"
	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update [app...]",
	Short: "Update apps to latest versions",
	Long: `Update Frappe apps to their latest versions.

By default, updates all apps. Specify app names to update specific apps.
This pulls the latest code, updates dependencies, and rebuilds assets.

Examples:
  weg update              # Update all apps
  weg update frappe       # Update only frappe
  weg update --pull       # Only pull, skip dependency install
  weg update --no-build   # Skip asset rebuild`,
	RunE: runUpdate,
}

var (
	pullOnly bool
	noBuild  bool
	noDeps   bool
)

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().BoolVar(&pullOnly, "pull", false, "Only pull latest code, skip dependency install")
	updateCmd.Flags().BoolVar(&noBuild, "no-build", false, "Skip asset rebuild after update")
	updateCmd.Flags().BoolVar(&noDeps, "no-deps", false, "Skip dependency installation")
}

func runUpdate(cmd *cobra.Command, args []string) error {
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

	var benchPath, appsDir string
	switch result.Context {
	case config.ContextWegApp:
		benchPath = filepath.Join(absPath, ".weg")
		appsDir = filepath.Join(benchPath, "apps")
	case config.ContextWegBench:
		benchPath = absPath
		appsDir = filepath.Join(benchPath, "apps")
	default:
		return fmt.Errorf("not a weg-managed project. Run 'weg init' first")
	}

	// Load state to get installed apps
	st, err := state.Load(absPath)
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	// Determine which apps to update
	appsToUpdate := args
	if len(appsToUpdate) == 0 {
		// Update all installed apps
		for name := range st.Apps {
			appsToUpdate = append(appsToUpdate, name)
		}
	}

	if len(appsToUpdate) == 0 {
		PrintInfo("No apps installed. Run 'weg sync' first.")
		return nil
	}

	PrintInfo("Updating %d app(s)...", len(appsToUpdate))
	startTime := time.Now()

	// Update apps in parallel
	var wg sync.WaitGroup
	results := make(chan updateResult, len(appsToUpdate))

	opts := apps.InstallOptions{
		BenchPath: benchPath,
		AppsDir:   appsDir,
		Verbose:   IsVerbose(),
	}

	for _, appName := range appsToUpdate {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()

			appStart := time.Now()
			err := updateSingleApp(name, opts)
			duration := time.Since(appStart)

			results <- updateResult{
				App:      name,
				Duration: duration,
				Error:    err,
			}
		}(appName)
	}

	// Wait and collect results
	go func() {
		wg.Wait()
		close(results)
	}()

	var failed []string
	for result := range results {
		if result.Error != nil {
			PrintError("%s: %v", result.App, result.Error)
			failed = append(failed, result.App)
		} else {
			PrintInfo("  %s updated in %v", result.App, result.Duration.Round(time.Millisecond))
		}
	}

	// Rebuild assets unless --no-build
	if !noBuild && !pullOnly && len(failed) == 0 {
		PrintInfo("\nRebuilding assets...")
		if err := runBuildAll(benchPath); err != nil {
			PrintError("Asset rebuild failed: %v", err)
		}
	}

	totalDuration := time.Since(startTime)

	if len(failed) > 0 {
		return fmt.Errorf("%d app(s) failed to update", len(failed))
	}

	PrintInfo("\nUpdate complete in %v", totalDuration.Round(time.Millisecond))
	return nil
}

type updateResult struct {
	App      string
	Duration time.Duration
	Error    error
}

func updateSingleApp(name string, opts apps.InstallOptions) error {
	appPath := filepath.Join(opts.AppsDir, name)

	// Check if app exists
	if !apps.IsGitRepo(appPath) {
		return fmt.Errorf("app not installed")
	}

	// Pull latest
	if err := apps.Pull(appPath); err != nil {
		return fmt.Errorf("git pull failed: %w", err)
	}

	if pullOnly {
		return nil
	}

	// Update dependencies unless --no-deps
	if !noDeps {
		if err := apps.InstallPythonDeps(appPath, opts); err != nil {
			return fmt.Errorf("pip install failed: %w", err)
		}

		if err := apps.InstallNodeDeps(appPath, opts); err != nil {
			// Node deps are optional, just log
			PrintVerbose("  %s: node deps skipped: %v", name, err)
		}
	}

	return nil
}

func runBuildAll(benchPath string) error {
	// Use bench build for now
	// TODO: Implement parallel esbuild
	return nil
}
