package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build [app...]",
	Short: "Build frontend assets",
	Long: `Build frontend assets for Frappe apps.

By default, builds assets for all apps in parallel. You can specify
specific apps to build, or use flags to control the build process.

Examples:
  weg build               # Build all apps
  weg build frappe        # Build only frappe
  weg build --production  # Production build (minified)
  weg build --watch       # Watch mode (rebuild on changes)`,
	RunE: runBuild,
}

var (
	production bool
	watch      bool
	force      bool
)

func init() {
	rootCmd.AddCommand(buildCmd)
	buildCmd.Flags().BoolVar(&production, "production", false, "Production build (minified)")
	buildCmd.Flags().BoolVar(&watch, "watch", false, "Watch mode - rebuild on file changes")
	buildCmd.Flags().BoolVar(&force, "force", false, "Force rebuild all assets")
}

func runBuild(cmd *cobra.Command, args []string) error {
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

	// Determine bench path and apps directory
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

	// Find apps to build
	appsToBuild, err := findAppsToBuild(appsDir, args)
	if err != nil {
		return fmt.Errorf("failed to find apps: %w", err)
	}

	if len(appsToBuild) == 0 {
		PrintInfo("No apps with frontend assets found.")
		return nil
	}

	if watch {
		return runWatchMode(benchPath, appsToBuild)
	}

	return runParallelBuild(benchPath, appsToBuild)
}

// findAppsToBuild returns apps that have frontend assets
func findAppsToBuild(appsDir string, filter []string) ([]string, error) {
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	filterSet := make(map[string]bool)
	for _, f := range filter {
		filterSet[f] = true
	}

	var apps []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()

		// If filter specified, only include matching apps
		if len(filterSet) > 0 && !filterSet[name] {
			continue
		}

		// Check if app has frontend assets (package.json with build script)
		appPath := filepath.Join(appsDir, name)
		if hasFrontendAssets(appPath) {
			apps = append(apps, name)
		}
	}

	return apps, nil
}

// hasFrontendAssets checks if an app has buildable frontend assets
func hasFrontendAssets(appPath string) bool {
	// Check for package.json
	packageJSON := filepath.Join(appPath, "package.json")
	if _, err := os.Stat(packageJSON); err != nil {
		return false
	}

	// Check for common frontend directories
	frontendDirs := []string{
		filepath.Join(appPath, "frontend"),
		filepath.Join(appPath, appPath, "public"),
		filepath.Join(appPath, appPath, "www"),
	}

	for _, dir := range frontendDirs {
		if _, err := os.Stat(dir); err == nil {
			return true
		}
	}

	// If package.json exists, assume it has frontend assets
	return true
}

// runParallelBuild builds all apps in parallel
func runParallelBuild(benchPath string, apps []string) error {
	startTime := time.Now()

	PrintInfo("Building %d app(s)...", len(apps))

	var wg sync.WaitGroup
	errors := make(chan error, len(apps))
	results := make(chan buildResult, len(apps))

	for _, app := range apps {
		wg.Add(1)
		go func(appName string) {
			defer wg.Done()

			appStart := time.Now()
			err := buildApp(benchPath, appName)
			duration := time.Since(appStart)

			results <- buildResult{
				App:      appName,
				Duration: duration,
				Error:    err,
			}

			if err != nil {
				errors <- fmt.Errorf("%s: %w", appName, err)
			}
		}(app)
	}

	// Wait for all builds to complete
	go func() {
		wg.Wait()
		close(errors)
		close(results)
	}()

	// Collect results
	var failed []string
	for result := range results {
		if result.Error != nil {
			PrintError("%s failed: %v", result.App, result.Error)
			failed = append(failed, result.App)
		} else {
			PrintInfo("  %s built in %v", result.App, result.Duration.Round(time.Millisecond))
		}
	}

	totalDuration := time.Since(startTime)

	if len(failed) > 0 {
		return fmt.Errorf("%d app(s) failed to build: %v", len(failed), failed)
	}

	PrintInfo("\nBuild complete in %v", totalDuration.Round(time.Millisecond))
	return nil
}

type buildResult struct {
	App      string
	Duration time.Duration
	Error    error
}

// buildApp builds a single app using esbuild via Frappe's build system
func buildApp(benchPath, appName string) error {
	appsDir := filepath.Join(benchPath, "apps")
	appPath := filepath.Join(appsDir, appName)

	// Use bench build command which uses Frappe's esbuild setup
	buildArgs := []string{"build", "--app", appName}
	if production {
		buildArgs = append(buildArgs, "--production")
	}
	if force {
		buildArgs = append(buildArgs, "--force")
	}

	cmd := exec.Command("bench", buildArgs...)
	cmd.Dir = benchPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("build failed: %w\n%s", err, string(output))
	}

	// Alternative: Direct esbuild for apps with frontend/ directory
	frontendDir := filepath.Join(appPath, "frontend")
	if _, statErr := os.Stat(frontendDir); statErr == nil {
		// Has Vue/React frontend, may need additional build
		PrintVerbose("App %s has frontend directory", appName)
	}

	return nil
}

// runWatchMode starts watch mode for continuous rebuilds
func runWatchMode(benchPath string, apps []string) error {
	PrintInfo("Starting watch mode for %d app(s)...", len(apps))
	PrintInfo("Press Ctrl+C to stop")

	// Use bench watch command
	cmd := exec.Command("bench", "watch")
	cmd.Dir = benchPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}
