package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gavindsouza/weg/internal/apps"
	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/runtime"
	"github.com/gavindsouza/weg/internal/services"
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
			analysis, err := updateSingleApp(name, opts)
			duration := time.Since(appStart)

			results <- updateResult{
				App:           name,
				Duration:      duration,
				Error:         err,
				NeedsMigrate:  analysis.NeedsMigrate,
				NeedsRestart:  analysis.NeedsRestart,
				NeedsPyDeps:   analysis.NeedsPyDeps,
				NeedsNodeDeps: analysis.NeedsNodeDeps,
			}
		}(appName)
	}

	// Wait and collect results
	go func() {
		wg.Wait()
		close(results)
	}()

	var failed []string
	anyNeedsMigrate := false
	anyNeedsRestart := false
	for result := range results {
		if result.Error != nil {
			PrintError("%s: %v", result.App, result.Error)
			failed = append(failed, result.App)
		} else {
			notes := []string{}
			if result.NeedsMigrate {
				notes = append(notes, "schema changes")
				anyNeedsMigrate = true
			}
			if result.NeedsRestart {
				notes = append(notes, "python changes")
				anyNeedsRestart = true
			}
			if result.NeedsPyDeps {
				notes = append(notes, "py deps updated")
			}
			if result.NeedsNodeDeps {
				notes = append(notes, "node deps updated")
			}
			noteStr := ""
			if len(notes) > 0 {
				noteStr = fmt.Sprintf(" (%s)", strings.Join(notes, ", "))
			}
			PrintInfo("  %s updated in %v%s", result.App, result.Duration.Round(time.Millisecond), noteStr)
		}
	}

	if len(failed) > 0 {
		return fmt.Errorf("%d app(s) failed to update", len(failed))
	}

	// Run migrate if schema changes detected
	if anyNeedsMigrate {
		PrintInfo("\nRunning migrations (schema changes detected)...")
		if err := runMigrateForUpdate(benchPath); err != nil {
			return fmt.Errorf("migrate failed: %w", err)
		}
	}

	// Rebuild assets unless --no-build
	if !noBuild && !pullOnly {
		PrintInfo("\nRebuilding assets...")
		if err := runBuildAll(benchPath); err != nil {
			PrintError("Asset rebuild failed: %v", err)
		}
	}

	// Restart services only if Python files changed (JS handled by watcher)
	if anyNeedsRestart && isServicesRunning(benchPath) {
		PrintInfo("\nRestarting services (Python changes detected)...")
		mgr := services.NewManager(benchPath)
		if rtConfig, err := runtime.Load(benchPath); err == nil {
			mgr.RunID = rtConfig.RunID
		}
		_ = mgr.Stop()
		_ = runtime.Remove(benchPath)
		PrintInfo("Services stopped. Run 'weg start' to restart.")
	} else if !anyNeedsRestart && isServicesRunning(benchPath) {
		PrintInfo("\nNo restart needed (only JS/CSS changes - handled by watcher)")
	}

	totalDuration := time.Since(startTime)
	PrintInfo("\nUpdate complete in %v", totalDuration.Round(time.Millisecond))
	return nil
}

type updateResult struct {
	App           string
	Duration      time.Duration
	Error         error
	NeedsMigrate  bool
	NeedsRestart  bool
	NeedsPyDeps   bool
	NeedsNodeDeps bool
}

type changeAnalysis struct {
	NeedsMigrate  bool
	NeedsRestart  bool
	NeedsPyDeps   bool
	NeedsNodeDeps bool
}

func updateSingleApp(name string, opts apps.InstallOptions) (analysis changeAnalysis, err error) {
	appPath := filepath.Join(opts.AppsDir, name)

	// Check if app exists
	if !apps.IsGitRepo(appPath) {
		return analysis, fmt.Errorf("app not installed")
	}

	// Get current HEAD before pull
	oldHead, _ := getGitHead(appPath)

	// Pull latest
	if err := apps.Pull(appPath); err != nil {
		return analysis, fmt.Errorf("git pull failed: %w", err)
	}

	// Get new HEAD after pull
	newHead, _ := getGitHead(appPath)

	// Check what changed
	if oldHead != "" && newHead != "" && oldHead != newHead {
		analysis = analyzeChanges(appPath, oldHead, newHead)
	}

	if pullOnly {
		return analysis, nil
	}

	// Update dependencies only if needed (unless --no-deps)
	if !noDeps {
		if analysis.NeedsPyDeps {
			PrintVerbose("  %s: installing Python deps (pyproject.toml changed)", name)
			if err := apps.InstallPythonDeps(appPath, opts); err != nil {
				return analysis, fmt.Errorf("pip install failed: %w", err)
			}
		}

		if analysis.NeedsNodeDeps {
			PrintVerbose("  %s: installing Node deps (package.json changed)", name)
			if err := apps.InstallNodeDeps(appPath, opts); err != nil {
				// Node deps are optional, just log
				PrintVerbose("  %s: node deps skipped: %v", name, err)
			}
		}
	}

	return analysis, nil
}

// getGitHead returns the current HEAD commit hash
func getGitHead(repoPath string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// analyzeChanges determines what type of changes occurred between commits
func analyzeChanges(repoPath, oldCommit, newCommit string) changeAnalysis {
	var analysis changeAnalysis

	cmd := exec.Command("git", "diff", "--name-only", oldCommit, newCommit)
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return analysis
	}

	changedFiles := strings.TrimSpace(string(output))
	if changedFiles == "" {
		return analysis
	}

	for _, file := range strings.Split(changedFiles, "\n") {
		basename := filepath.Base(file)

		// Schema changes requiring migrate
		if strings.HasSuffix(file, ".json") {
			if strings.Contains(file, "/doctype/") ||
				strings.Contains(file, "/workspace/") ||
				strings.Contains(file, "/custom/") ||
				strings.Contains(file, "/fixtures/") {
				analysis.NeedsMigrate = true
			}
		}

		// Python changes requiring restart
		if strings.HasSuffix(file, ".py") {
			analysis.NeedsRestart = true
		}

		// Python dependency changes
		if basename == "pyproject.toml" || basename == "setup.py" || basename == "requirements.txt" {
			analysis.NeedsPyDeps = true
		}

		// Node dependency changes
		if basename == "package.json" {
			analysis.NeedsNodeDeps = true
		}
	}

	return analysis
}

// runMigrateForUpdate runs database migrations during update
func runMigrateForUpdate(benchPath string) error {
	sitesDir := filepath.Join(benchPath, "sites")
	shellCmd := fmt.Sprintf("cd %s && ../.venv/bin/python -m frappe.utils.bench_helper frappe --site all migrate", sitesDir)

	cmd := exec.Command("devbox", "run", "-c", benchPath, "--", "sh", "-c", shellCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// isServicesRunning checks if weg services are currently running
func isServicesRunning(benchPath string) bool {
	_, err := runtime.Load(benchPath)
	return err == nil
}

func runBuildAll(benchPath string) error {
	// Run frappe build via bench_helper
	sitesDir := filepath.Join(benchPath, "sites")
	shellCmd := fmt.Sprintf("cd %s && ../.venv/bin/python -m frappe.utils.bench_helper frappe build", sitesDir)

	buildCmd := exec.Command("devbox", "run", "-c", benchPath, "--", "sh", "-c", shellCmd)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr

	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	return nil
}
