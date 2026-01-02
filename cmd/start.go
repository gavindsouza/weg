package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/runtime"
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

Services run in the background by default. Use 'weg stop' to stop them.

Examples:
  weg start              # Start services in background
  weg start -f           # Start with TUI (foreground)
  weg start --no-watch   # Start without file watcher`,
	RunE: runStart,
}

var (
	foreground bool
	noWatch    bool
	noSync     bool
)

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().BoolVarP(&foreground, "foreground", "f", false, "Run in foreground with TUI")
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

	// Load bench config to get preferred ports
	benchConfig, _ := config.ParseWegToml(benchPath)

	// Get preferred ports from config or use defaults
	preferredPorts := runtime.DefaultPorts()
	if benchConfig != nil {
		if benchConfig.Services.Web.Port > 0 {
			preferredPorts.Web = benchConfig.Services.Web.Port
		}
		if benchConfig.Services.Web.SocketPort > 0 {
			preferredPorts.SocketIO = benchConfig.Services.Web.SocketPort
		}
	}

	// Allocate available ports
	ports, err := runtime.AllocatePorts(preferredPorts)
	if err != nil {
		return fmt.Errorf("failed to allocate ports: %w", err)
	}

	// Save runtime config
	rtConfig := &runtime.Config{
		Ports: *ports,
		PID:   os.Getpid(),
	}
	if err := rtConfig.Save(benchPath); err != nil {
		PrintVerbose("Warning: failed to save runtime config: %v", err)
	}

	// Show allocated ports if different from defaults
	if ports.Web != preferredPorts.Web {
		PrintInfo("Port %d in use, using %d for web server", preferredPorts.Web, ports.Web)
	}
	if ports.SocketIO != preferredPorts.SocketIO {
		PrintInfo("Port %d in use, using %d for socket.io", preferredPorts.SocketIO, ports.SocketIO)
	}

	// Generate/update process-compose.yaml with allocated ports
	opts := services.DefaultComposeOptions(benchPath)
	opts.IncludeWatch = !noWatch
	opts.WebPort = ports.Web
	opts.SocketPort = ports.SocketIO

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

	// Update common_site_config.json with runtime ports from weg.toml config
	if err := updateRuntimeSiteConfig(benchPath, benchConfig, ports); err != nil {
		PrintVerbose("Warning: failed to update site config with ports: %v", err)
	}

	// Start services
	mgr := services.NewManager(benchPath)
	mgr.Verbose = IsVerbose()
	mgr.ProcessComposePort = ports.ProcessCompose

	if foreground {
		PrintInfo("Starting services on port %d... (Ctrl+C to stop)", ports.Web)
		return mgr.Start()
	}

	// Default: run detached
	PrintInfo("Starting services in background...")
	if err := mgr.StartDetached(); err != nil {
		return err
	}
	PrintInfo("Services started on port %d. Use 'weg stop' to stop them.", ports.Web)
	return nil
}

// updateRuntimeSiteConfig updates common_site_config.json with runtime port values
// Uses weg.toml config as base and overrides with runtime ports
func updateRuntimeSiteConfig(benchPath string, benchConfig *config.BenchConfig, ports *runtime.Ports) error {
	configPath := filepath.Join(benchPath, "sites", "common_site_config.json")

	// Generate config from weg.toml with runtime ports
	runtimePorts := &config.RuntimePorts{
		Web:      ports.Web,
		SocketIO: ports.SocketIO,
	}

	var cfg map[string]interface{}
	if benchConfig != nil {
		cfg = benchConfig.GenerateCommonSiteConfig(runtimePorts)
	} else {
		// Fallback to defaults with runtime ports
		cfg = map[string]interface{}{
			"redis_cache":    "redis://localhost:6379/0",
			"redis_queue":    "redis://localhost:6379/1",
			"redis_socketio": "redis://localhost:6379/2",
			"webserver_port": ports.Web,
			"socketio_port":  ports.SocketIO,
			"developer_mode": 1,
		}
	}

	// Write config
	newData, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, newData, 0644)
}
