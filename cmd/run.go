package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gavindsouza/weg/internal/apps"
	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/services"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/gavindsouza/weg/tools"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run <app-url>",
	Short: "Clone and run a Frappe app instantly",
	Long: `Clone a Frappe app repository and run it with zero configuration.

This is the magic runner - give it a git URL and it will:
1. Clone the repository
2. Detect if it's a Frappe app (looks for hooks.py)
3. Read pyproject.toml [tool.weg] for compatibility info
4. Prompt for Frappe version and database (if not specified)
5. Create a temporary development environment
6. Install all dependencies
7. Create a site and install the app
8. Start the development server

Examples:
  weg run https://github.com/frappe/erpnext
  weg run https://github.com/frappe/hrms --version 15
  weg run git@github.com:myorg/myapp.git --db postgres
  weg run frappe/hrms --branch version-15
  weg run ./local-app-directory

When to use which: 'weg run' spins up a disposable environment for an
existing app; use 'weg new' to create your own app and 'weg init' to set
up a permanent environment for an existing checkout.`,
	Args: cobra.ExactArgs(1),
	RunE: runRun,
}

var (
	runVersion   string
	runDatabase  string
	runBranch    string
	runKeep      bool
	runDirectory string
	runSkipDeps  bool
)

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().StringVar(&runVersion, "version", "", "Frappe version to use (14, 15, 16)")
	runCmd.Flags().StringVar(&runDatabase, "db", "", "Database to use (mariadb, postgres, sqlite)")
	runCmd.Flags().StringVarP(&runBranch, "branch", "b", "", "Branch or tag to clone")
	runCmd.Flags().BoolVar(&runKeep, "keep", false, "Keep the environment after exit (don't cleanup)")
	runCmd.Flags().StringVar(&runDirectory, "dir", "", "Directory to create environment in (default: temp)")
	runCmd.Flags().BoolVar(&runSkipDeps, "skip-deps", false, "Skip dependency resolution; install only frappe and the target app")
}

func runRun(cmd *cobra.Command, args []string) error {
	// Resolve app spec using shared resolver
	appSpec := apps.ResolveAppSpec(args[0], runBranch)

	PrintInfo("Weg Magic Runner")
	PrintInfo("==================")
	PrintInfo("")

	// Step 1: Determine if it's a local path or git URL
	var appPath string
	var appName string
	var isTemp bool
	var err error

	appName = appSpec.Name

	if appSpec.IsLocal {
		// Local directory
		appPath = appSpec.URL
		PrintInfo("Using local app: %s", appPath)
	} else {
		if runDirectory != "" {
			appPath = filepath.Join(runDirectory, appName)
		} else {
			tempDir, err := os.MkdirTemp("", "weg-run-*")
			if err != nil {
				return fmt.Errorf("failed to create temp directory: %w", err)
			}
			appPath = filepath.Join(tempDir, appName)
			isTemp = true
		}

		source := appSpec.URL
		if source == "" {
			return errors.Usagef("cannot resolve app URL for %q — use a full URL or org/repo format", appName)
		}

		PrintInfo("Cloning %s...", appSpec)
		if err := apps.CloneRepo(source, appSpec.Branch, appPath, false); err != nil {
			return fmt.Errorf("failed to clone repository: %w", err)
		}
		PrintInfo("Cloned to: %s", appPath)
	}

	// Step 2: Detect if it's a Frappe app
	hooksPath := filepath.Join(appPath, appName, "hooks.py")
	if _, err := os.Stat(hooksPath); os.IsNotExist(err) {
		// Try without nested directory
		hooksPath = filepath.Join(appPath, "hooks.py")
		if _, err := os.Stat(hooksPath); os.IsNotExist(err) {
			return errors.Usage("not a Frappe app (hooks.py not found)")
		}
	}
	PrintInfo("Detected Frappe app: %s", appName)

	// Step 3: Read pyproject.toml for compatibility info
	var appConfig *config.AppConfig
	pyprojectPath := filepath.Join(appPath, "pyproject.toml")
	if _, err := os.Stat(pyprojectPath); err == nil {
		appConfig, err = config.ParsePyproject(pyprojectPath)
		if err != nil {
			PrintVerbose("Warning: failed to parse pyproject.toml: %v", err)
		}
	}

	// Step 4: Determine Frappe version
	frappeVersion := runVersion
	if frappeVersion == "" {
		if appConfig != nil && len(appConfig.Compatibility.Frappe) > 0 {
			// Use the latest compatible version
			frappeVersion = appConfig.Compatibility.Frappe[len(appConfig.Compatibility.Frappe)-1]
			PrintInfo("Using compatible Frappe version: %s", frappeVersion)
		} else {
			// Prompt for version
			frappeVersion, err = promptVersion()
			if err != nil {
				return err
			}
		}
	}

	// Step 5: Determine database
	database := runDatabase
	if database == "" {
		if appConfig != nil && len(appConfig.Compatibility.Databases) > 0 {
			database = appConfig.Compatibility.Databases[0]
			PrintInfo("Using compatible database: %s", database)
		} else {
			// Default based on version
			if frappeVersion == "16" {
				database = "sqlite"
			} else {
				database = "mariadb"
			}
			PrintInfo("Using default database: %s", database)
		}
	}

	// Validate database support
	if !tools.IsDatabaseSupported(frappeVersion, database) {
		return errors.Validation("database", fmt.Sprintf("%s is not supported for Frappe %s", database, frappeVersion))
	}

	// Step 6: Create .weg environment
	wegPath := filepath.Join(appPath, ".weg")
	PrintInfo("")
	PrintInfo("Setting up development environment...")

	if err := os.MkdirAll(wegPath, 0755); err != nil {
		return fmt.Errorf("failed to create .weg directory: %w", err)
	}

	// Create apps and sites directories
	appsDir := filepath.Join(wegPath, "apps")
	sitesDir := filepath.Join(wegPath, "sites")
	os.MkdirAll(appsDir, 0755)
	os.MkdirAll(sitesDir, 0755)

	// Create weg.toml
	siteName := fmt.Sprintf("%s.localhost", appName)
	wegToml := fmt.Sprintf(`# Generated by weg run
[bench]
name = "%s-dev"

[frappe]
version = "%s"
database = "%s"

[apps.frappe]
url = "https://github.com/frappe/frappe"
branch = "version-%s"

[apps.%s]
path = ".."

[[sites]]
name = "%s"
default = true
apps = ["frappe", "%s"]
`, appName, frappeVersion, database, frappeVersion, appName, siteName, appName)

	wegTomlPath := filepath.Join(wegPath, "weg.toml")
	if err := os.WriteFile(wegTomlPath, []byte(wegToml), 0644); err != nil {
		return errors.Config("weg.toml", "write", err)
	}

	// Step 7: Install Frappe
	PrintInfo("Installing Frappe %s...", frappeVersion)

	installOpts := apps.InstallOptions{
		BenchPath:     wegPath,
		AppsDir:       appsDir,
		FrappeVersion: frappeVersion,
		Verbose:       verbose,
	}

	frappeURL := "https://github.com/frappe/frappe"
	frappeBranch := fmt.Sprintf("version-%s", frappeVersion)

	if err := apps.InstallApp("frappe", frappeURL, frappeBranch, installOpts); err != nil {
		return fmt.Errorf("failed to install frappe: %w", err)
	}

	// Initialize state
	st := state.NewState()
	st.Apps["frappe"] = state.AppState{
		Name:        "frappe",
		URL:         frappeURL,
		Branch:      frappeBranch,
		InstalledAt: time.Now(),
	}

	// Link the app (it's a local path)
	PrintInfo("Linking %s...", appName)
	appLink := filepath.Join(appsDir, appName)
	if err := os.Symlink(appPath, appLink); err != nil && !os.IsExist(err) {
		// If symlink fails, try relative path
		relPath, _ := filepath.Rel(appsDir, appPath)
		if err := os.Symlink(relPath, appLink); err != nil && !os.IsExist(err) {
			PrintVerbose("Warning: failed to symlink app: %v", err)
		}
	}

	// Install app Python dependencies
	if err := apps.InstallPythonDeps(appLink, installOpts); err != nil {
		PrintVerbose("Warning: failed to install Python deps for %s: %v", appName, err)
	}

	st.Apps[appName] = state.AppState{
		Name:        appName,
		Path:        appPath,
		InstalledAt: time.Now(),
	}

	// Resolve and install transitive dependencies (unless --skip-deps)
	if !runSkipDeps {
		PrintInfo("Resolving dependencies for %s...", appName)
		installed := map[string]bool{"frappe": true, appName: true}

		resolveOpts := apps.ResolveOptions{
			BenchPath:     wegPath,
			AppsDir:       appsDir,
			AllowRemote:   true,
			InstalledApps: installed,
			Verbose:       true,
			LogFunc: func(format string, a ...any) {
				PrintInfo(format, a...)
			},
		}

		resolveResult, err := apps.ResolveDependencies(appSpec, resolveOpts)
		if err != nil {
			PrintVerbose("Dependency resolution failed: %v — continuing without transitive deps", err)
		} else if len(resolveResult.InstallOrder) > 0 {
			apps.PrintResolveResult(resolveResult)

			// Add deps to the weg.toml and install them
			for _, dep := range resolveResult.InstallOrder {
				PrintInfo("Installing dependency: %s...", dep.Name)
				depBranch := dep.Branch
				if depBranch == "" {
					depBranch = fmt.Sprintf("version-%s", frappeVersion)
				}
				if err := apps.InstallApp(dep.Name, dep.URL, depBranch, installOpts); err != nil {
					PrintVerbose("Warning: failed to install dependency %s: %v", dep.Name, err)
					continue
				}
				st.Apps[dep.Name] = state.AppState{
					Name:        dep.Name,
					URL:         dep.URL,
					Branch:      depBranch,
					InstalledAt: time.Now(),
				}
				PrintInfo("Installed dependency: %s", dep.Name)
			}
		} else {
			PrintInfo("No additional dependencies to resolve")
		}
	}

	// Create site
	PrintInfo("Creating site %s...", siteName)
	if err := os.MkdirAll(filepath.Join(sitesDir, siteName), 0755); err != nil {
		return fmt.Errorf("failed to create site directory: %w", err)
	}

	// Generate site_config.json
	siteConfig := fmt.Sprintf(`{
  "db_name": "%s",
  "db_type": "%s"
}`, strings.ReplaceAll(appName, "-", "_"), getDatabaseType(database))

	siteConfigPath := filepath.Join(sitesDir, siteName, "site_config.json")
	if err := os.WriteFile(siteConfigPath, []byte(siteConfig), 0644); err != nil {
		PrintVerbose("Warning: failed to write site_config.json: %v", err)
	}

	// Set as current site
	currentSitePath := filepath.Join(sitesDir, "currentsite.txt")
	os.WriteFile(currentSitePath, []byte(siteName), 0644)

	st.AddSite(state.SiteState{
		Name:        siteName,
		DefaultSite: true,
		Apps:        []string{"frappe", appName},
	})

	// Save state
	if err := st.Save(wegPath); err != nil {
		output.Warningf("Failed to save state: %v", err)
	}

	// Step 8: Start the server
	PrintInfo("")
	PrintInfo("Environment ready!")
	PrintInfo("")
	PrintInfo("Starting development server...")
	PrintInfo("Press Ctrl+C to stop")
	PrintInfo("")

	// Generate process-compose.yaml
	composeOpts := services.DefaultComposeOptions(wegPath)
	composeConfig := services.GenerateProcessCompose(composeOpts)
	if err := services.WriteProcessCompose(wegPath, composeConfig); err != nil {
		PrintVerbose("Warning: failed to write process-compose.yaml: %v", err)
	}

	// Set up signal handling for cleanup
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start process-compose
	composePath := filepath.Join(wegPath, "process-compose.yaml")
	pcCmd := exec.Command("process-compose", "up", "-f", composePath)
	pcCmd.Dir = wegPath
	pcCmd.Stdout = os.Stdout
	pcCmd.Stderr = os.Stderr
	pcCmd.Stdin = os.Stdin

	if err := pcCmd.Start(); err != nil {
		// Fallback: just run bench serve
		PrintInfo("process-compose not found, falling back to bench serve...")
		benchCmd := exec.Command("bench", "serve", "--port", "8000")
		benchCmd.Dir = wegPath
		benchCmd.Stdout = os.Stdout
		benchCmd.Stderr = os.Stderr
		benchCmd.Stdin = os.Stdin

		go func() {
			<-sigChan
			benchCmd.Process.Signal(syscall.SIGTERM)
		}()

		if err := benchCmd.Run(); err != nil {
			PrintVerbose("Server exited: %v", err)
		}
	} else {
		go func() {
			<-sigChan
			pcCmd.Process.Signal(syscall.SIGTERM)
		}()

		pcCmd.Wait()
	}

	// Cleanup
	PrintInfo("")
	if isTemp && !runKeep {
		PrintInfo("Cleaning up temporary environment...")
		os.RemoveAll(filepath.Dir(appPath))
		PrintInfo("Done!")
	} else if !runKeep {
		PrintInfo("Environment kept at: %s", wegPath)
		PrintInfo("Run again with: weg run %s", appPath)
	}

	return nil
}

func promptVersion() (string, error) {
	if yes {
		return "15", nil // Default to v15
	}

	fmt.Print("Select Frappe version [14/15/16] (default: 15): ")
	reader := bufio.NewReader(os.Stdin)
	answer, err := reader.ReadString('\n')
	if err != nil {
		return "15", nil
	}
	answer = strings.TrimSpace(answer)

	if answer == "" {
		return "15", nil
	}

	if answer != "14" && answer != "15" && answer != "16" {
		return "", errors.Validation("version", fmt.Sprintf("must be 14, 15, or 16, got %s", answer))
	}

	return answer, nil
}

func getDatabaseType(db string) string {
	switch db {
	case "postgres":
		return "postgres"
	case "sqlite":
		return "sqlite"
	default:
		return "mariadb"
	}
}
