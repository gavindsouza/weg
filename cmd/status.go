package cmd

import (
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/runtime"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the current state of the weg environment",
	Long: `Display the current status of the weg-managed environment.

Shows:
  - Detected context (app or bench)
  - Installed apps and their versions
  - Configured sites
  - Whether sync is needed

Examples:
  weg status         # Show status of current directory
  weg status -v      # Show verbose status with more details`,
	RunE: runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
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

	// Print header
	output.Print("Weg Status")
	output.Printf("==========\n")

	// Context info
	output.Printf("Context:  %s", result.ContextDescription())
	output.Printf("Path:     %s", absPath)
	if result.BenchPath != "" {
		output.Printf("Bench:    %s", result.BenchPath)
	}
	if result.ConfigPath != "" {
		output.Printf("Config:   %s", result.ConfigPath)
	}
	if result.AppName != "" {
		output.Printf("App:      %s", result.AppName)
	}

	// Show default site if in a managed project
	if result.IsWegManaged() {
		if site := ResolveDefaultSite(absPath); site != "" {
			output.Printf("Site:     %s (default)", site)
		}
	}

	// Warn about ambiguous configuration signals
	if result.IsWegManaged() {
		if config.HasWegToml(absPath) && config.HasWegSection(absPath) {
			output.Warningf("Both weg.toml and pyproject.toml [tool.weg] detected.")
			output.Printf("  weg.toml takes precedence. Remove one to avoid confusion.")
		}
	}

	// Handle different contexts
	switch result.Context {
	case config.ContextFresh:
		output.Printf("\nThis directory is not initialized.")
		output.Print("Run 'weg init' to get started.")
		return nil

	case config.ContextApp:
		output.Printf("\nThis is a Frappe app without weg configuration.")
		output.Print("Run 'weg init' to add weg management.")
		return nil

	case config.ContextBench:
		output.Printf("\nThis is a traditional bench without weg management.")
		output.Print("Run 'weg init' to import into weg.")
		return nil

	case config.ContextWegApp:
		return showAppStatus(absPath, result)

	case config.ContextWegBench:
		return showBenchStatus(absPath, result)
	}

	return nil
}

func showAppStatus(path string, result *config.DetectionResult) error {
	output.Printf("\n--- App Configuration ---\n")

	// Parse pyproject.toml
	appConfig, err := config.ParsePyproject(path)
	if err != nil {
		PrintVerbose("Could not parse pyproject.toml: %v", err)
	} else {
		output.Print("Compatibility:")
		output.Printf("  Frappe:     %v", appConfig.Compatibility.Frappe)
		output.Printf("  Databases:  %v", appConfig.Compatibility.Databases)
		output.Printf("\nDevelopment:")
		output.Printf("  Frappe:     %s", appConfig.Dev.Frappe)
		output.Printf("  Database:   %s", appConfig.Dev.Database)

		if len(appConfig.Dependencies.Apps) > 0 {
			output.Printf("\nDependencies:")
			for _, dep := range appConfig.Dependencies.Apps {
				output.Printf("  - %s", dep.Name)
			}
		}
	}

	// Show state
	return showStateInfo(path)
}

func showBenchStatus(path string, result *config.DetectionResult) error {
	output.Printf("\n--- Bench Configuration ---\n")

	// Parse weg.toml
	benchConfig, err := config.ParseWegToml(path)
	if err != nil {
		PrintVerbose("Could not parse weg.toml: %v", err)
	} else {
		output.Printf("Bench:      %s", benchConfig.Bench.Name)
		output.Printf("Frappe:     %s", benchConfig.Frappe.Version)
		output.Printf("Database:   %s", benchConfig.Frappe.Database)

		output.Printf("\nApps (%d configured):", len(benchConfig.Apps))
		for name, app := range benchConfig.Apps {
			status := ""
			if app.Excluded {
				status = " (excluded)"
			}
			if app.URL != "" {
				output.Printf("  - %s @ %s%s", name, app.Branch, status)
			} else if app.Path != "" {
				output.Printf("  - %s (local: %s)%s", name, app.Path, status)
			}
		}

		if len(benchConfig.Sites) > 0 {
			output.Printf("\nSites (%d configured):", len(benchConfig.Sites))
			for _, site := range benchConfig.Sites {
				defaultMark := ""
				if site.DefaultSite {
					defaultMark = " (default)"
				}
				output.Printf("  - %s%s", site.Name, defaultMark)
			}
		}

		// Show worker configuration
		showWorkerConfig(benchConfig.Services.Workers)
	}

	// Show runtime status if services are running
	showRuntimeStatus(path)

	// Show state
	return showStateInfo(path)
}

func showStateInfo(path string) error {
	output.Printf("\n--- Current State ---\n")

	st, err := state.Load(path)
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	if st.IsEmpty() {
		output.Print("No state recorded. Run 'weg sync' to initialize.")
		return nil
	}

	output.Printf("Last sync: %s", formatTime(st.LastSync))

	if len(st.Apps) > 0 {
		output.Printf("\nInstalled Apps (%d):", len(st.Apps))
		for name, app := range st.Apps {
			branch := app.Branch
			if branch == "" {
				branch = "local"
			}
			commit := ""
			if app.Commit != "" && IsVerbose() {
				commit = fmt.Sprintf(" [%s]", app.Commit[:7])
			}
			output.Printf("  - %s @ %s%s", name, branch, commit)
		}
	}

	if len(st.Sites) > 0 {
		output.Printf("\nActive Sites (%d):", len(st.Sites))
		for name, site := range st.Sites {
			defaultMark := ""
			if site.DefaultSite {
				defaultMark = " (default)"
			}
			apps := ""
			if len(site.Apps) > 0 && IsVerbose() {
				apps = fmt.Sprintf(" [apps: %v]", site.Apps)
			}
			output.Printf("  - %s%s%s", name, defaultMark, apps)
		}
	}

	// Check if sync is needed
	configPath := filepath.Join(path, "weg.toml")
	if !config.HasWegToml(path) {
		configPath = filepath.Join(path, "pyproject.toml")
	}

	needsSync, err := st.NeedsSync(configPath)
	if err != nil {
		PrintVerbose("Could not check sync status: %v", err)
	} else if needsSync {
		output.Print("")
		PrintInfo("Configuration has changed since last sync.")
		PrintInfo("Run 'weg sync' to apply changes.")
	} else {
		output.Print("")
		PrintInfo("Environment is in sync with configuration.")
	}

	return nil
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "never"
	}

	now := time.Now()
	diff := now.Sub(t)

	if diff < time.Minute {
		return "just now"
	} else if diff < time.Hour {
		mins := int(diff.Minutes())
		return fmt.Sprintf("%d minute(s) ago", mins)
	} else if diff < 24*time.Hour {
		hours := int(diff.Hours())
		return fmt.Sprintf("%d hour(s) ago", hours)
	}

	return t.Format("2006-01-02 15:04")
}

func showWorkerConfig(workers map[string]int) {
	output.Printf("\nWorkers:")

	if len(workers) == 0 {
		output.Print("  1 worker (all queues) [default]")
		return
	}

	// Sort queue names for consistent output
	queues := make([]string, 0, len(workers))
	for q := range workers {
		queues = append(queues, q)
	}
	sort.Strings(queues)

	totalWorkers := 0
	for _, queue := range queues {
		count := workers[queue]
		if count <= 0 {
			continue
		}
		totalWorkers += count

		queueType := "dedicated"
		queueDesc := queue
		if queue == "all" {
			queueType = "shared"
			queueDesc = "short,default,long"
		}

		if count == 1 {
			output.Printf("  %d worker (%s) [%s]", count, queueDesc, queueType)
		} else {
			output.Printf("  %d workers (%s) [%s]", count, queueDesc, queueType)
		}
	}

	output.Printf("  Total: %d worker(s)", totalWorkers)
}

func showRuntimeStatus(path string) {
	rtConfig, err := runtime.LoadIfRunning(path)
	if err != nil || rtConfig == nil {
		return
	}

	output.Printf("\n--- Runtime Status ---\n")
	output.Print("Status:   running")
	output.Printf("Web:      http://localhost:%d", rtConfig.Ports.Web)
	output.Printf("SocketIO: port %d", rtConfig.Ports.SocketIO)
	if rtConfig.RunID != "" {
		output.Printf("Run ID:   %s", rtConfig.RunID)
	}
}
