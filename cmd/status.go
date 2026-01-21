package cmd

import (
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/gavindsouza/weg/internal/config"
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
	result, err := config.DetectContext(absPath)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	// Print header
	fmt.Printf("Weg Status\n")
	fmt.Printf("==========\n\n")

	// Context info
	fmt.Printf("Context:  %s\n", result.ContextDescription())
	fmt.Printf("Path:     %s\n", absPath)
	if result.ConfigPath != "" {
		fmt.Printf("Config:   %s\n", result.ConfigPath)
	}
	if result.AppName != "" {
		fmt.Printf("App:      %s\n", result.AppName)
	}

	// Handle different contexts
	switch result.Context {
	case config.ContextFresh:
		fmt.Printf("\nThis directory is not initialized.\n")
		fmt.Printf("Run 'weg init' to get started.\n")
		return nil

	case config.ContextApp:
		fmt.Printf("\nThis is a Frappe app without weg configuration.\n")
		fmt.Printf("Run 'weg init' to add weg management.\n")
		return nil

	case config.ContextBench:
		fmt.Printf("\nThis is a traditional bench without weg management.\n")
		fmt.Printf("Run 'weg init' to import into weg.\n")
		return nil

	case config.ContextWegApp:
		return showAppStatus(absPath, result)

	case config.ContextWegBench:
		return showBenchStatus(absPath, result)
	}

	return nil
}

func showAppStatus(path string, result *config.DetectionResult) error {
	fmt.Printf("\n--- App Configuration ---\n\n")

	// Parse pyproject.toml
	appConfig, err := config.ParsePyproject(path)
	if err != nil {
		PrintVerbose("Could not parse pyproject.toml: %v", err)
	} else {
		fmt.Printf("Compatibility:\n")
		fmt.Printf("  Frappe:     %v\n", appConfig.Compatibility.Frappe)
		fmt.Printf("  Databases:  %v\n", appConfig.Compatibility.Databases)
		fmt.Printf("\nDevelopment:\n")
		fmt.Printf("  Frappe:     %s\n", appConfig.Dev.Frappe)
		fmt.Printf("  Database:   %s\n", appConfig.Dev.Database)

		if len(appConfig.Dependencies.Apps) > 0 {
			fmt.Printf("\nDependencies:\n")
			for _, dep := range appConfig.Dependencies.Apps {
				fmt.Printf("  - %s\n", dep.Name)
			}
		}
	}

	// Show state
	return showStateInfo(path)
}

func showBenchStatus(path string, result *config.DetectionResult) error {
	fmt.Printf("\n--- Bench Configuration ---\n\n")

	// Parse weg.toml
	benchConfig, err := config.ParseWegToml(path)
	if err != nil {
		PrintVerbose("Could not parse weg.toml: %v", err)
	} else {
		fmt.Printf("Bench:      %s\n", benchConfig.Bench.Name)
		fmt.Printf("Frappe:     %s\n", benchConfig.Frappe.Version)
		fmt.Printf("Database:   %s\n", benchConfig.Frappe.Database)

		fmt.Printf("\nApps (%d configured):\n", len(benchConfig.Apps))
		for name, app := range benchConfig.Apps {
			status := ""
			if app.Excluded {
				status = " (excluded)"
			}
			if app.URL != "" {
				fmt.Printf("  - %s @ %s%s\n", name, app.Branch, status)
			} else if app.Path != "" {
				fmt.Printf("  - %s (local: %s)%s\n", name, app.Path, status)
			}
		}

		if len(benchConfig.Sites) > 0 {
			fmt.Printf("\nSites (%d configured):\n", len(benchConfig.Sites))
			for _, site := range benchConfig.Sites {
				defaultMark := ""
				if site.DefaultSite {
					defaultMark = " (default)"
				}
				fmt.Printf("  - %s%s\n", site.Name, defaultMark)
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
	fmt.Printf("\n--- Current State ---\n\n")

	st, err := state.Load(path)
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	if st.IsEmpty() {
		fmt.Printf("No state recorded. Run 'weg sync' to initialize.\n")
		return nil
	}

	fmt.Printf("Last sync: %s\n", formatTime(st.LastSync))

	if len(st.Apps) > 0 {
		fmt.Printf("\nInstalled Apps (%d):\n", len(st.Apps))
		for name, app := range st.Apps {
			branch := app.Branch
			if branch == "" {
				branch = "local"
			}
			commit := ""
			if app.Commit != "" && IsVerbose() {
				commit = fmt.Sprintf(" [%s]", app.Commit[:7])
			}
			fmt.Printf("  - %s @ %s%s\n", name, branch, commit)
		}
	}

	if len(st.Sites) > 0 {
		fmt.Printf("\nActive Sites (%d):\n", len(st.Sites))
		for name, site := range st.Sites {
			defaultMark := ""
			if site.DefaultSite {
				defaultMark = " (default)"
			}
			apps := ""
			if len(site.Apps) > 0 && IsVerbose() {
				apps = fmt.Sprintf(" [apps: %v]", site.Apps)
			}
			fmt.Printf("  - %s%s%s\n", name, defaultMark, apps)
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
		fmt.Println()
		PrintInfo("Configuration has changed since last sync.")
		PrintInfo("Run 'weg sync' to apply changes.")
	} else {
		fmt.Println()
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
	fmt.Printf("\nWorkers:\n")

	if len(workers) == 0 {
		fmt.Printf("  1 worker (all queues) [default]\n")
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
			fmt.Printf("  %d worker (%s) [%s]\n", count, queueDesc, queueType)
		} else {
			fmt.Printf("  %d workers (%s) [%s]\n", count, queueDesc, queueType)
		}
	}

	fmt.Printf("  Total: %d worker(s)\n", totalWorkers)
}

func showRuntimeStatus(path string) {
	rtConfig, err := runtime.LoadIfRunning(path)
	if err != nil || rtConfig == nil {
		return
	}

	fmt.Printf("\n--- Runtime Status ---\n\n")
	fmt.Printf("Status:   running\n")
	fmt.Printf("Web:      http://localhost:%d\n", rtConfig.Ports.Web)
	fmt.Printf("SocketIO: port %d\n", rtConfig.Ports.SocketIO)
	if rtConfig.RunID != "" {
		fmt.Printf("Run ID:   %s\n", rtConfig.RunID)
	}
}
