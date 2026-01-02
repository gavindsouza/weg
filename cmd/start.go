package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/services"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start development services",
	Long: `Start all development services for the current project.

This command starts:
  - MariaDB (database)
  - Redis (cache and queue)
  - Web server (Gunicorn)
  - Socket.io server
  - Background workers
  - Scheduler
  - File watcher (for auto-rebuild)

Services are managed by devbox services and process-compose.
Press Ctrl+C to stop all services.

Examples:
  weg start              # Start all services
  weg start --detach     # Start in background
  weg start --no-watch   # Start without file watcher`,
	RunE: runStart,
}

var (
	detach  bool
	noWatch bool
	noSync  bool
)

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().BoolVarP(&detach, "detach", "d", false, "Run services in background")
	startCmd.Flags().BoolVar(&noWatch, "no-watch", false, "Disable file watcher")
	startCmd.Flags().BoolVar(&noSync, "no-sync", false, "Skip sync check before starting")
}

func runStart(cmd *cobra.Command, args []string) error {
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
		return fmt.Errorf("not a weg-managed project. Run 'weg init' first")
	}

	// Check if sync is needed (unless --no-sync)
	if !noSync {
		st, err := state.Load(absPath)
		if err == nil && !st.IsEmpty() {
			configPath := result.ConfigPath
			if configPath == "" {
				if config.HasWegToml(absPath) {
					configPath = filepath.Join(absPath, "weg.toml")
				} else {
					configPath = filepath.Join(absPath, "pyproject.toml")
				}
			}

			needsSync, _ := st.NeedsSync(configPath)
			if needsSync {
				PrintInfo("Configuration has changed. Running sync first...")
				// TODO: Call sync logic here
			}
		}
	}

	// Generate/update process-compose.yaml
	opts := services.DefaultComposeOptions(benchPath)
	opts.IncludeWatch = !noWatch

	// For devbox projects, don't include redis (devbox services handles it)
	// and use .venv Python for bench commands
	devboxJSON := filepath.Join(benchPath, "devbox.json")
	if _, err := os.Stat(devboxJSON); err == nil {
		opts.IncludeRedis = false
		opts.UseVenvPython = true
	}

	if err := services.GenerateAndWrite(opts); err != nil {
		return fmt.Errorf("failed to generate process-compose.yaml: %w", err)
	}

	// Start services
	mgr := services.NewManager(benchPath)
	mgr.Verbose = IsVerbose()

	if detach {
		PrintInfo("Starting services in background...")
		if err := mgr.StartDetached(); err != nil {
			return err
		}
		PrintInfo("Services started. Use 'weg stop' to stop them.")
		return nil
	}

	PrintInfo("Starting services... (Ctrl+C to stop)")
	return mgr.Start()
}
