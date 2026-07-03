package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/tools"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	frappeVersion string
	appsJSON      string
)

var createCmd = &cobra.Command{
	Use:   "create [bench-name]",
	Short: "Create a new Frappe bench with Devbox",
	Long: `Create a new Frappe bench with a Devbox-managed environment.

Sets up a traditional bench structure (apps/, sites/) and clones frappe
plus any requested apps.

Examples:
  weg create mybench
  weg create mybench --version version-15

When to use which: 'weg create' builds a new bench; use 'weg new' for a
brand-new app, 'weg init' for an existing project, and 'weg run' to try
out an existing app.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		frappe := tools.FrappeApp{
			Name:   "frappe",
			Url:    "https://github.com/frappe/frappe",
			Branch: frappeVersion,
		}
		otherApps := tools.ParseAppsJSON(appsJSON)
		apps := append([]tools.FrappeApp{frappe}, otherApps...)
		err := create(args[0], apps)
		if err != nil {
			output.Errorf("%s", err)
			os.Exit(1)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(createCmd)
	// No shorthand: -v is taken by the global --verbose flag and would panic.
	createCmd.Flags().StringVar(&frappeVersion, "version", "develop", "Frappe version to use")
	createCmd.Flags().StringVar(&appsJSON, "apps", "[]", "JSON of apps to initialize bench with")
}

// getAppPaths returns a slice of app paths for the given apps
func getAppPaths(benchPath string, apps []tools.FrappeApp) []string {
	var paths []string
	for _, app := range apps {
		paths = append(paths, filepath.Join(benchPath, "apps", app.Name))
	}
	return paths
}

func create(benchPath string, apps []tools.FrappeApp) error {
	startTime := time.Now()
	success := false
	defer func() {
		if !success {
			output.Printf("\nBench path: %s", benchPath)
		}
	}()

	tools.DebugLog("Starting bench creation process")
	pm := tools.NewProgressManager()

	// Initialize progress bars for each major step
	pm.AddBar("Creating directory structure", 4) // apps, sites, config/pids, logs
	pm.AddBar("Setting up environment", 2)       // .envrc, devbox init
	pm.AddBar("Installing dependencies", 1)      // devbox add
	pm.AddBar("Cloning Frappe apps", len(apps))
	pm.AddBar("Setting up Python environment", 2)         // pyproject.toml, uv venv
	pm.AddBar("Installing app dependencies", len(apps)*2) // N apps * 2 (uv add + yarn install)
	pm.AddBar("Setting up bench config", 3)               // config, redis, procfile

	tools.DebugLog("Creating directory structure")
	// Create directory structure
	if err := createDirectoryStructure(benchPath, pm); err != nil {
		return fmt.Errorf("failed to create directory structure: %w", err)
	}

	tools.DebugLog("Creating .envrc")
	// Create .envrc
	if err := createEnvrc(benchPath, pm); err != nil {
		return fmt.Errorf("failed to create .envrc: %w", err)
	}

	tools.DebugLog("Initializing Devbox")
	// Initialize Devbox
	if err := tools.RunCmdWithError("devbox", benchPath, "init"); err != nil {
		return fmt.Errorf("failed to initialize devbox: %w", err)
	}
	pm.Increment("Setting up environment")
	pm.Finish("Setting up environment")

	ORIGINAL_PATH := os.Getenv("PATH")
	os.Setenv("PATH", fmt.Sprintf("%s/.devbox/bin:%s/.devbox/nix/profile/default/bin:%s", benchPath, benchPath, ORIGINAL_PATH))

	// Get packages and apps before starting goroutines
	var packages, _ = tools.GetDependencies(frappeVersion)

	tools.DebugLog("Starting parallel operations (dependencies and git clone)")
	// Start both devbox installation and git clone in parallel
	var wg sync.WaitGroup
	errChan := make(chan error, 2)

	// Install dependencies in a goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		var devboxArgs []string = []string{"uv"}

		for _, pkg := range packages {
			if pkg.Version != "" {
				devboxArgs = append(devboxArgs, fmt.Sprintf("%s@%s", pkg.Name, pkg.Version))
			} else {
				devboxArgs = append(devboxArgs, pkg.Name)
			}
		}

		if err := tools.RunCmdWithError("devbox", benchPath, append([]string{"add"}, devboxArgs...)...); err != nil {
			errChan <- fmt.Errorf("failed to install dependencies: %w", err)
			return
		}
		pm.Finish("Installing dependencies")
		errChan <- nil
	}()

	// Clone Frappe apps in a goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := tools.CloneRepos(apps, filepath.Join(benchPath, "apps")); err != nil {
			errChan <- fmt.Errorf("failed to clone repos: %w", err)
			return
		}
		pm.Finish("Cloning Frappe apps")
		errChan <- nil
	}()

	// Wait for both operations to complete and check for errors
	if err := tools.WaitForErrors(&wg, errChan); err != nil {
		return err
	}

	tools.DebugLog("Entering devbox shell")
	if err := tools.RunCmdWithError("devbox", benchPath, "shell"); err != nil {
		return fmt.Errorf("failed to enter devbox shell: %w", err)
	}

	tools.DebugLog("Creating pyproject.toml")
	// Create pyproject.toml
	if err := createPyproject(benchPath, packages, pm); err != nil {
		return fmt.Errorf("failed to create pyproject.toml: %w", err)
	}

	tools.DebugLog("Allowing direnv")
	if err := tools.RunCmdWithError("direnv", benchPath, "allow"); err != nil {
		return fmt.Errorf("failed to allow direnv: %w", err)
	}

	tools.DebugLog("Creating virtual environment")
	if err := tools.RunCmdWithError("uv", benchPath, "venv", "env", "--python", benchPath+"/.devbox/nix/profile/default/bin/python"); err != nil {
		return fmt.Errorf("failed to create virtual environment: %w", err)
	}
	pm.Increment("Setting up Python environment")
	pm.Finish("Setting up Python environment")

	tools.DebugLog("Creating apps.txt")
	// Write apps.txt
	if err := createAppsTxt(benchPath, apps); err != nil {
		return fmt.Errorf("failed to create apps.txt: %w", err)
	}

	// Create wait groups for both app dependencies and bench setup
	var appWg sync.WaitGroup
	var setupWg sync.WaitGroup
	appErrChan := make(chan error, 8)   // 4 apps * 2 operations
	setupErrChan := make(chan error, 2) // 2 setup commands (redis and procfile)

	tools.DebugLog("Setting up bench config")
	// First run bench setup config
	if err := tools.RunCmdWithError("bench", benchPath, "setup", "config"); err != nil {
		return fmt.Errorf("failed to setup bench config: %w", err)
	}
	pm.Increment("Setting up bench config")

	tools.DebugLog("Starting parallel bench setup commands")
	// Now start the remaining bench setup commands in parallel
	// setupWg.Add(2)
	tools.RunAsync(&setupWg, setupErrChan, pm, "bench", benchPath, []string{"setup", "redis"}, "Setting up bench config")

	// NOTE: Remove setup procfile in favour of using devbox services
	// tools.RunAsync(&setupWg, setupErrChan, pm, "bench", benchPath, []string{"setup", "procfile"}, "Setting up bench config")

	tools.DebugLog("Setting up app dependencies")
	// Setup requirements js & python
	appPaths := getAppPaths(benchPath, apps)
	uvArgs := append([]string{"add", "--active", "--editable"}, appPaths...)

	// Run uv command asynchronously
	appWg.Add(1)
	go func() {
		defer appWg.Done()
		if err := tools.RunCmdWithError("uv", benchPath, uvArgs...); err != nil {
			appErrChan <- fmt.Errorf("failed to install Python dependencies: %w", err)
			return
		}
		// Increment progress for each app
		for i := 0; i < len(apps); i++ {
			pm.Increment("Installing app dependencies")
		}
		appErrChan <- nil
	}()

	// Run yarn install for each app asynchronously
	for _, app := range apps {
		tools.RunAsync(&appWg, appErrChan, pm, "yarn", filepath.Join(benchPath, "apps", app.Name), []string{"install"}, "Installing app dependencies")
	}

	tools.DebugLog("Waiting for app dependencies to complete")
	// Wait for all async commands to complete and check for errors
	if err := tools.WaitForErrors(&appWg, appErrChan); err != nil {
		return err
	}
	pm.Finish("Installing app dependencies")

	tools.DebugLog("Waiting for bench setup to complete")
	// Wait for bench setup commands to complete and check for errors
	if err := tools.WaitForErrors(&setupWg, setupErrChan); err != nil {
		return err
	}
	pm.Finish("Setting up bench config")

	tools.DebugLog("Bench creation completed successfully")
	duration := time.Since(startTime)
	output.Printf("\nBench created in %v at %s", duration.Round(time.Second), benchPath)

	if err := writeOrUpdateDevboxServices(benchPath); err != nil {
		return fmt.Errorf("failed to write devbox.yaml: %w", err)
	}

	success = true
	return nil
}

func createDirectoryStructure(benchPath string, pm *tools.ProgressManager) error {
	if err := os.MkdirAll(benchPath, 0755); err != nil {
		return fmt.Errorf("failed to create bench directory: %w", err)
	}
	pm.Increment("Creating directory structure")

	if err := os.MkdirAll(filepath.Join(benchPath, "apps"), 0755); err != nil {
		return fmt.Errorf("failed to create apps directory: %w", err)
	}
	pm.Increment("Creating directory structure")

	if err := os.MkdirAll(filepath.Join(benchPath, "sites"), 0755); err != nil {
		return fmt.Errorf("failed to create sites directory: %w", err)
	}
	pm.Increment("Creating directory structure")

	if err := os.MkdirAll(filepath.Join(benchPath, "config/pids"), 0755); err != nil {
		return fmt.Errorf("failed to create config/pids directory: %w", err)
	}
	pm.Increment("Creating directory structure")

	if err := os.MkdirAll(filepath.Join(benchPath, "logs"), 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}
	pm.Finish("Creating directory structure")
	return nil
}

func createEnvrc(benchPath string, pm *tools.ProgressManager) error {
	envrcFile, err := os.Create(filepath.Join(benchPath, ".envrc"))
	if err != nil {
		return fmt.Errorf("failed to create .envrc file: %w", err)
	}
	defer envrcFile.Close()

	var envrcContent = `# This file is generated by the create command. Do not edit manually.
# Use 'direnv allow' to allow this file.
export VENV_DIR="$(pwd)/env"
export UV_PYTHON=$VENV_DIR/bin/python
export UV_VENV_PATH=$VENV_DIR
export VIRTUAL_ENV=$VENV_DIR
eval "$(devbox generate direnv --print-envrc -e VENV_DIR=$VENV_DIR -e UV_PYTHON=$UV_PYTHON -e UV_VENV_PATH=$UV_VENV_PATH -e VIRTUAL_ENV=$VIRTUAL_ENV)"
`
	os.Setenv("VENV_DIR", fmt.Sprintf("%s/env", benchPath))
	os.Setenv("VIRTUAL_ENV", fmt.Sprintf("%s/env", benchPath))
	os.Setenv("UV_PYTHON", fmt.Sprintf("%s/bin/python", os.Getenv("VENV_DIR")))
	os.Setenv("UV_VENV_PATH", os.Getenv("VENV_DIR"))

	if _, err := envrcFile.WriteString(envrcContent); err != nil {
		return fmt.Errorf("failed to write to .envrc file: %w", err)
	}
	pm.Increment("Setting up environment")
	return nil
}

func createPyproject(benchPath string, packages []tools.Dependency, pm *tools.ProgressManager) error {
	pyprojectFile, err := os.Create(filepath.Join(benchPath, "pyproject.toml"))
	if err != nil {
		return fmt.Errorf("failed to create pyproject.toml file: %w", err)
	}
	defer pyprojectFile.Close()

	var pyprojectContent = fmt.Sprintf(`# This file is generated by the create command. Do not edit manually.
# Use 'devbox add' to add dependencies.
[project]
name = "frappe-bench"
version = "0.1.0"
requires-python = ">=%s"
`, packages[slices.IndexFunc(packages, func(pkg tools.Dependency) bool {
		return pkg.Name == "python"
	})].Version)

	if _, err := pyprojectFile.WriteString(pyprojectContent); err != nil {
		return fmt.Errorf("failed to write to pyproject.toml file: %w", err)
	}
	pm.Increment("Setting up Python environment")
	return nil
}

func createAppsTxt(benchPath string, apps []tools.FrappeApp) error {
	appsFile, err := os.Create(filepath.Join(benchPath, "sites/apps.txt"))
	if err != nil {
		return fmt.Errorf("failed to create apps.txt file: %w", err)
	}
	defer appsFile.Close()

	for _, app := range apps {
		if _, err := appsFile.WriteString(app.Name + "\n"); err != nil {
			return fmt.Errorf("failed to write to apps.txt file: %w", err)
		}
	}
	return nil
}

func writeOrUpdateDevboxServices(benchPath string) error {
	devboxFile := filepath.Join(benchPath, "process-compose.yaml")

	// Define the services
	services := map[string]map[string]any{
		"redis":       {"disabled": true},
		"redis_cache": {"command": "redis-server config/redis_cache.conf"},
		"redis_queue": {"command": "redis-server config/redis_queue.conf"},
		"mariadb": {
			"command":  "mysqld_safe --socket=" + filepath.Join(benchPath, "data/mariadb.sock") + " --skip-networking --port=0 --console",
			"disabled": false, // Disabled by default, user can enable manually
		},
		"web":      {"command": "bench serve --port 8000"},
		"socketio": {"command": filepath.Join(benchPath, ".devbox/nix/profile/default/bin/node") + " apps/frappe/socketio.js"},
		"watch":    {"command": "bench watch"},
		"schedule": {"command": "bench schedule"},
		"worker":   {"command": "bench worker 1>> logs/worker.log 2>> logs/worker.error.log"},
	}

	var devboxConfig map[string]any

	// If devbox.yaml exists, read and unmarshal it
	if _, err := os.Stat(devboxFile); err == nil {
		f, err := os.Open(devboxFile)
		if err != nil {
			return err
		}
		defer f.Close()
		if err := yaml.NewDecoder(f).Decode(&devboxConfig); err != nil {
			return err
		}
	} else {
		devboxConfig = make(map[string]any)
	}

	// Set or update the services
	devboxConfig["processes"] = services

	// Write back to devbox.yaml
	f, err := os.Create(devboxFile)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := yaml.NewEncoder(f)
	defer enc.Close()
	return enc.Encode(devboxConfig)
}
