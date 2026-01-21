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
  weg run ./local-app-directory`,
	Args: cobra.ExactArgs(1),
	RunE: runRun,
}

var (
	runVersion   string
	runDatabase  string
	runKeep      bool
	runDirectory string
)

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().StringVar(&runVersion, "version", "", "Frappe version to use (14, 15, 16)")
	runCmd.Flags().StringVar(&runDatabase, "db", "", "Database to use (mariadb, postgres, sqlite)")
	runCmd.Flags().BoolVar(&runKeep, "keep", false, "Keep the environment after exit (don't cleanup)")
	runCmd.Flags().StringVar(&runDirectory, "dir", "", "Directory to create environment in (default: temp)")
}

func runRun(cmd *cobra.Command, args []string) error {
	appURL := args[0]

	PrintInfo("Weg Magic Runner")
	PrintInfo("==================")
	PrintInfo("")

	// Step 1: Determine if it's a local path or git URL
	var appPath string
	var appName string
	var isTemp bool
	var err error

	if isLocalPath(appURL) {
		// Local directory
		appPath, err = filepath.Abs(appURL)
		if err != nil {
			return fmt.Errorf("invalid path: %w", err)
		}
		appName = filepath.Base(appPath)
		PrintInfo("Using local app: %s", appPath)
	} else {
		// Git URL - clone to temp directory
		appName = extractAppNameFromURL(appURL)

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

		PrintInfo("Cloning %s...", appURL)
		if err := apps.CloneRepo(appURL, "", appPath, false); err != nil {
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
			return fmt.Errorf("not a Frappe app (hooks.py not found)")
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
		return fmt.Errorf("database %s is not supported for Frappe %s", database, frappeVersion)
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
		return fmt.Errorf("failed to write weg.toml: %w", err)
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
	st.Save(wegPath)

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

func isLocalPath(path string) bool {
	// Check if it's a local path
	if strings.HasPrefix(path, "/") || strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") {
		return true
	}
	if strings.HasPrefix(path, "~") {
		return true
	}
	// Check if the path exists locally
	if _, err := os.Stat(path); err == nil {
		return true
	}
	return false
}

func extractAppNameFromURL(url string) string {
	// Extract app name from git URL
	// https://github.com/frappe/erpnext -> erpnext
	// git@github.com:frappe/erpnext.git -> erpnext

	url = strings.TrimSuffix(url, ".git")
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		name := parts[len(parts)-1]
		// Handle git@ URLs
		if strings.Contains(name, ":") {
			colonParts := strings.Split(name, ":")
			if len(colonParts) > 1 {
				name = colonParts[len(colonParts)-1]
			}
		}
		return name
	}
	return "app"
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
		return "", fmt.Errorf("invalid version: %s (must be 14, 15, or 16)", answer)
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
