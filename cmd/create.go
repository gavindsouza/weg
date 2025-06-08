package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"

	"github.com/gavindsouza/weg/tools"
	"github.com/spf13/cobra"
)

var frappeVersion string

var createCmd = &cobra.Command{
	Use:   "create [bench-name]",
	Short: "Create a new Frappe bench with Devbox",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return create(args[0])
	},
}

func init() {
	rootCmd.AddCommand(createCmd)
	createCmd.Flags().StringVar(&frappeVersion, "frappe-version", "develop", "Frappe version to use")
}

// debugLog prints debug messages only when WEG_NO_PROGRESS is set
func debugLog(format string, args ...interface{}) {
	if os.Getenv("WEG_NO_PROGRESS") != "" {
		fmt.Printf("DEBUG: "+format+"\n", args...)
	}
}

// runAsync runs a command asynchronously and handles errors
func runAsync(wg *sync.WaitGroup, errChan chan<- error, pm *tools.ProgressManager, binary, dir string, args []string, progressBar string) {
	debugLog("Starting async command: %s %v in %s", binary, args, dir)
	wg.Add(1)
	go func() {
		defer func() {
			debugLog("Async command completed: %s %v", binary, args)
			wg.Done()
		}()

		if err := runCmdWithError(binary, dir, args...); err != nil {
			debugLog("Async command failed: %s %v: %v", binary, args, err)
			errChan <- fmt.Errorf("failed to run %s: %w", binary, err)
			return
		}

		if progressBar != "" {
			debugLog("Incrementing progress bar: %s", progressBar)
			pm.Increment(progressBar)
		}
		errChan <- nil
	}()
}

// waitForErrors waits for all goroutines to complete and checks for errors
func waitForErrors(wg *sync.WaitGroup, errChan chan error) error {
	debugLog("Starting waitForErrors")
	wg.Wait()
	debugLog("All goroutines completed")
	close(errChan)
	debugLog("Error channel closed")

	var errors []error
	for err := range errChan {
		if err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		debugLog("Found %d errors", len(errors))
		for _, err := range errors {
			debugLog("Error: %v", err)
		}
		return fmt.Errorf("encountered %d errors: %v", len(errors), errors)
	}

	debugLog("No errors found")
	return nil
}

// getAppPaths returns a slice of app paths for the given apps
func getAppPaths(benchPath string, apps []tools.FrappeApp) []string {
	var paths []string
	for _, app := range apps {
		paths = append(paths, filepath.Join(benchPath, "apps", app.Name))
	}
	return paths
}

func create(benchPath string) error {
	debugLog("Starting bench creation process")
	pm := tools.NewProgressManager()

	// Initialize progress bars for each major step
	pm.AddBar("Creating directory structure", 4)  // apps, sites, config/pids, logs
	pm.AddBar("Setting up environment", 2)        // .envrc, devbox init
	pm.AddBar("Installing dependencies", 1)       // devbox add
	pm.AddBar("Cloning Frappe apps", 4)           // frappe, erpnext, hrms, raven
	pm.AddBar("Setting up Python environment", 2) // pyproject.toml, uv venv
	pm.AddBar("Installing app dependencies", 8)   // 4 apps * 2 (uv add + yarn install)
	pm.AddBar("Setting up bench config", 3)       // config, redis, procfile

	debugLog("Creating directory structure")
	// Create directory structure
	if err := createDirectoryStructure(benchPath, pm); err != nil {
		return fmt.Errorf("failed to create directory structure: %w", err)
	}

	debugLog("Creating .envrc")
	// Create .envrc
	if err := createEnvrc(benchPath, pm); err != nil {
		return fmt.Errorf("failed to create .envrc: %w", err)
	}

	debugLog("Initializing Devbox")
	// Initialize Devbox
	if err := runCmdWithError("devbox", benchPath, "init"); err != nil {
		return fmt.Errorf("failed to initialize devbox: %w", err)
	}
	pm.Increment("Setting up environment")
	pm.Finish("Setting up environment")

	ORIGINAL_PATH := os.Getenv("PATH")
	os.Setenv("PATH", fmt.Sprintf("%s/.devbox/bin:%s/.devbox/nix/profile/default/bin:%s", benchPath, benchPath, ORIGINAL_PATH))

	// Get packages and apps before starting goroutines
	var packages, _ = tools.GetDependencies(frappeVersion)
	var apps = []tools.FrappeApp{
		{Url: "https://github.com/frappe/frappe.git", Name: "frappe", Branch: "develop"},
		{Url: "https://github.com/frappe/erpnext.git", Name: "erpnext", Branch: "develop"},
		{Url: "https://github.com/frappe/hrms.git", Name: "hrms", Branch: "develop"},
		{Url: "https://github.com/The-Commit-Company/Raven.git", Name: "raven", Branch: "develop"},
	}

	debugLog("Starting parallel operations (dependencies and git clone)")
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

		if err := runCmdWithError("devbox", benchPath, append([]string{"add"}, devboxArgs...)...); err != nil {
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
	if err := waitForErrors(&wg, errChan); err != nil {
		return err
	}

	debugLog("Entering devbox shell")
	if err := runCmdWithError("devbox", benchPath, "shell"); err != nil {
		return fmt.Errorf("failed to enter devbox shell: %w", err)
	}

	debugLog("Creating pyproject.toml")
	// Create pyproject.toml
	if err := createPyproject(benchPath, packages, pm); err != nil {
		return fmt.Errorf("failed to create pyproject.toml: %w", err)
	}

	debugLog("Allowing direnv")
	if err := runCmdWithError("direnv", benchPath, "allow"); err != nil {
		return fmt.Errorf("failed to allow direnv: %w", err)
	}

	debugLog("Creating virtual environment")
	if err := runCmdWithError("uv", benchPath, "venv", "env", "--python", benchPath+"/.devbox/nix/profile/default/bin/python"); err != nil {
		return fmt.Errorf("failed to create virtual environment: %w", err)
	}
	pm.Increment("Setting up Python environment")
	pm.Finish("Setting up Python environment")

	debugLog("Creating apps.txt")
	// Write apps.txt
	if err := createAppsTxt(benchPath, apps); err != nil {
		return fmt.Errorf("failed to create apps.txt: %w", err)
	}

	// Create wait groups for both app dependencies and bench setup
	var appWg sync.WaitGroup
	var setupWg sync.WaitGroup
	appErrChan := make(chan error, 8)   // 4 apps * 2 operations
	setupErrChan := make(chan error, 2) // 2 setup commands (redis and procfile)

	debugLog("Setting up bench config")
	// First run bench setup config
	if err := runCmdWithError("bench", benchPath, "setup", "config"); err != nil {
		return fmt.Errorf("failed to setup bench config: %w", err)
	}
	pm.Increment("Setting up bench config")

	debugLog("Starting parallel bench setup commands")
	// Now start the remaining bench setup commands in parallel
	// setupWg.Add(2)
	runAsync(&setupWg, setupErrChan, pm, "bench", benchPath, []string{"setup", "redis"}, "Setting up bench config")
	runAsync(&setupWg, setupErrChan, pm, "bench", benchPath, []string{"setup", "procfile"}, "Setting up bench config")

	debugLog("Setting up app dependencies")
	// Setup requirements js & python
	appPaths := getAppPaths(benchPath, apps)
	uvArgs := append([]string{"add", "--active", "--editable"}, appPaths...)

	// Run uv command asynchronously
	appWg.Add(1)
	go func() {
		defer appWg.Done()
		if err := runCmdWithError("uv", benchPath, uvArgs...); err != nil {
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
		runAsync(&appWg, appErrChan, pm, "yarn", filepath.Join(benchPath, "apps", app.Name), []string{"install"}, "Installing app dependencies")
	}

	debugLog("Waiting for app dependencies to complete")
	// Wait for all async commands to complete and check for errors
	if err := waitForErrors(&appWg, appErrChan); err != nil {
		return err
	}
	pm.Finish("Installing app dependencies")

	debugLog("Waiting for bench setup to complete")
	// Wait for bench setup commands to complete and check for errors
	if err := waitForErrors(&setupWg, setupErrChan); err != nil {
		return err
	}
	pm.Finish("Setting up bench config")

	debugLog("Bench creation completed successfully")
	fmt.Printf("\n\n✅ Bench created successfully at %s\n", benchPath)
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

func runCmdWithError(binary, dir string, args ...string) error {
	debugLog("Running command: %s %v in %s", binary, args, dir)
	cmd := exec.Command(binary, args...)
	cmd.Dir = dir

	// For bench commands, capture stderr for debugging
	if binary == "bench" {
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("command failed: %w\nstderr: %s", err, stderr.String())
		}
		debugLog("Command completed successfully: %s %v", binary, args)
		return nil
	}

	// For other commands, suppress output only if progress bars are enabled
	if os.Getenv("WEG_NO_PROGRESS") == "" {
		cmd.Stdout = nil
		cmd.Stderr = nil
	}
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("command failed: %w", err)
	}
	debugLog("Command completed successfully: %s %v", binary, args)
	return nil
}
