package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/gavindsouza/weg/tools"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [path]",
	Short: "Initialize weg in a directory",
	Long: `Initialize weg configuration based on detected context.

Weg init is smart - it detects what kind of directory you're in and
takes the appropriate action:

  Fresh directory     → Interactive setup (app or bench)
  Frappe app          → Add [tool.weg] to pyproject.toml
  Traditional bench   → Generate weg.toml from existing structure
  Weg-managed project → Show status and suggest next steps

Examples:
  weg init              # Initialize in current directory
  weg init ./myproject  # Initialize in specific directory
  weg init --bench      # Force bench-style initialization`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInit,
}

var (
	forceBench bool
	forceApp   bool
)

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVar(&forceBench, "bench", false, "Force bench-style initialization")
	initCmd.Flags().BoolVar(&forceApp, "app", false, "Force app-style initialization")
	initCmd.MarkFlagsMutuallyExclusive("bench", "app")
}

func runInit(cmd *cobra.Command, args []string) error {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Detect context
	result, err := config.DetectContext(absPath)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	PrintVerbose("Detected context: %s", result.Context.String())

	switch result.Context {
	case config.ContextFresh:
		return initFresh(absPath)
	case config.ContextApp:
		return initApp(absPath, result)
	case config.ContextBench:
		return initBench(absPath, result)
	case config.ContextWegApp, config.ContextWegBench:
		return showExistingStatus(absPath, result)
	default:
		return fmt.Errorf("unknown context: %s", result.Context.String())
	}
}

// initFresh handles initialization in an empty directory
func initFresh(path string) error {
	PrintInfo("Fresh directory detected at %s", path)

	if forceBench {
		return initFreshBench(path)
	}
	if forceApp {
		return initFreshApp(path)
	}

	// Interactive prompt
	if !AssumeYes() {
		fmt.Println("\nWhat would you like to create?")
		fmt.Println("  1. New Frappe app (app-centric development)")
		fmt.Println("  2. New bench (traditional multi-app setup)")
		fmt.Print("\nChoice [1]: ")

		reader := bufio.NewReader(os.Stdin)
		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		if choice == "2" {
			return initFreshBench(path)
		}
	}

	return initFreshApp(path)
}

// initFreshApp sets up a new app-centric project
func initFreshApp(path string) error {
	PrintInfo("Setting up app-centric project...")

	// Get app name
	appName := filepath.Base(path)
	if !AssumeYes() {
		fmt.Printf("App name [%s]: ", appName)
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input != "" {
			appName = input
		}
	}

	// Get Frappe version
	frappeVersion := "15"
	if !AssumeYes() {
		fmt.Printf("Frappe version (14/15/16) [%s]: ", frappeVersion)
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input != "" {
			frappeVersion = input
		}
	}

	// Get database
	database := "mariadb"
	if !AssumeYes() {
		dbs := "mariadb/postgres"
		if frappeVersion == "16" || frappeVersion == "develop" {
			dbs = "mariadb/postgres/sqlite"
		}
		fmt.Printf("Database (%s) [%s]: ", dbs, database)
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input != "" {
			database = input
		}
	}

	// Validate database choice
	if !tools.IsDatabaseSupported(frappeVersion, database) {
		return fmt.Errorf("database %s is not supported for Frappe %s", database, frappeVersion)
	}

	// Create pyproject.toml with [tool.weg]
	pyprojectPath := filepath.Join(path, "pyproject.toml")
	pyprojectContent := fmt.Sprintf(`[project]
name = "%s"
version = "0.0.1"
description = "A Frappe app"
requires-python = ">=3.10"

[tool.weg]
# Weg configuration for app-centric development

[tool.weg.compatibility]
frappe = ["%s"]
databases = ["%s"]

[tool.weg.dev]
frappe = "%s"
database = "%s"
`, appName, frappeVersion, database, frappeVersion, database)

	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(pyprojectPath, []byte(pyprojectContent), 0644); err != nil {
		return fmt.Errorf("failed to write pyproject.toml: %w", err)
	}

	PrintInfo("Created pyproject.toml with [tool.weg] configuration")
	PrintInfo("\nNext steps:")
	PrintInfo("  1. Run 'weg install' to set up the development environment")
	PrintInfo("  2. Run 'weg start' to start the development server")

	return nil
}

// initFreshBench sets up a new bench project
func initFreshBench(path string) error {
	PrintInfo("Setting up bench project...")

	// Get bench name
	benchName := filepath.Base(path)
	if !AssumeYes() {
		fmt.Printf("Bench name [%s]: ", benchName)
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input != "" {
			benchName = input
		}
	}

	// Get Frappe version
	frappeVersion := "15"
	if !AssumeYes() {
		fmt.Printf("Frappe version (14/15/16) [%s]: ", frappeVersion)
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input != "" {
			frappeVersion = input
		}
	}

	// Get database
	database := "mariadb"
	if !AssumeYes() {
		dbs := "mariadb/postgres"
		if frappeVersion == "16" || frappeVersion == "develop" {
			dbs = "mariadb/postgres/sqlite"
		}
		fmt.Printf("Database (%s) [%s]: ", dbs, database)
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input != "" {
			database = input
		}
	}

	// Validate database choice
	if !tools.IsDatabaseSupported(frappeVersion, database) {
		return fmt.Errorf("database %s is not supported for Frappe %s", database, frappeVersion)
	}

	// Create weg.toml
	wegPath := filepath.Join(path, "weg.toml")
	wegContent := fmt.Sprintf(`# Weg configuration for bench: %s

[bench]
name = "%s"

[frappe]
version = "%s"
database = "%s"

[apps]
# Add apps here, e.g.:
# erpnext = { url = "https://github.com/frappe/erpnext", branch = "version-15" }

[apps.frappe]
url = "https://github.com/frappe/frappe"
branch = "version-%s"

[[sites]]
name = "%s.localhost"
default = true
`, benchName, benchName, frappeVersion, database, frappeVersion, benchName)

	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(wegPath, []byte(wegContent), 0644); err != nil {
		return fmt.Errorf("failed to write weg.toml: %w", err)
	}

	PrintInfo("Created weg.toml configuration")
	PrintInfo("\nNext steps:")
	PrintInfo("  1. Edit weg.toml to add apps you want to install")
	PrintInfo("  2. Run 'weg sync' to set up the environment")
	PrintInfo("  3. Run 'weg start' to start the development server")

	return nil
}

// initApp handles initialization for an existing Frappe app
func initApp(path string, result *config.DetectionResult) error {
	PrintInfo("Frappe app detected: %s", result.AppName)

	// Check if pyproject.toml exists
	pyprojectPath := filepath.Join(path, "pyproject.toml")
	if _, err := os.Stat(pyprojectPath); os.IsNotExist(err) {
		// Create new pyproject.toml
		return initFreshApp(path)
	}

	// Read existing pyproject.toml and add [tool.weg] section
	PrintInfo("Adding [tool.weg] section to existing pyproject.toml...")

	// Get Frappe version
	frappeVersion := "15"
	if !AssumeYes() {
		fmt.Printf("Frappe version (14/15/16) [%s]: ", frappeVersion)
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input != "" {
			frappeVersion = input
		}
	}

	// Get database
	database := "mariadb"
	if !AssumeYes() {
		dbs := "mariadb/postgres"
		if frappeVersion == "16" || frappeVersion == "develop" {
			dbs = "mariadb/postgres/sqlite"
		}
		fmt.Printf("Database (%s) [%s]: ", dbs, database)
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input != "" {
			database = input
		}
	}

	// Append [tool.weg] section
	wegSection := fmt.Sprintf(`
[tool.weg]
# Weg configuration for app-centric development

[tool.weg.compatibility]
frappe = ["%s"]
databases = ["%s"]

[tool.weg.dev]
frappe = "%s"
database = "%s"
`, frappeVersion, database, frappeVersion, database)

	f, err := os.OpenFile(pyprojectPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open pyproject.toml: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(wegSection); err != nil {
		return fmt.Errorf("failed to write to pyproject.toml: %w", err)
	}

	PrintInfo("Added [tool.weg] section to pyproject.toml")
	PrintInfo("\nNext steps:")
	PrintInfo("  1. Run 'weg install' to set up the development environment")
	PrintInfo("  2. Run 'weg start' to start the development server")

	return nil
}

// initBench imports an existing bench into weg management
func initBench(path string, result *config.DetectionResult) error {
	PrintInfo("Traditional bench detected at %s", path)
	PrintInfo("Importing bench into weg management...")

	// Scan apps directory
	appsDir := filepath.Join(path, "apps")
	apps, err := scanAppsDirectory(appsDir)
	if err != nil {
		return fmt.Errorf("failed to scan apps: %w", err)
	}

	// Scan sites directory
	sitesDir := filepath.Join(path, "sites")
	sites, err := scanSitesDirectory(sitesDir)
	if err != nil {
		return fmt.Errorf("failed to scan sites: %w", err)
	}

	// Detect Frappe version from installed frappe app
	frappeVersion := detectFrappeVersion(filepath.Join(appsDir, "frappe"))

	// Generate weg.toml
	benchName := filepath.Base(path)
	wegContent := fmt.Sprintf(`# Weg configuration imported from existing bench
# Generated by 'weg init'

[bench]
name = "%s"

[frappe]
version = "%s"
database = "mariadb"

[apps]
`, benchName, frappeVersion)

	// Add apps
	for name, appInfo := range apps {
		if appInfo.URL != "" {
			wegContent += fmt.Sprintf(`
[apps.%s]
url = "%s"
branch = "%s"
`, name, appInfo.URL, appInfo.Branch)
		} else {
			wegContent += fmt.Sprintf(`
[apps.%s]
path = "%s"
`, name, appInfo.Path)
		}
	}

	// Add sites
	for i, site := range sites {
		defaultStr := ""
		if i == 0 {
			defaultStr = "\ndefault = true"
		}
		wegContent += fmt.Sprintf(`
[[sites]]
name = "%s"%s
`, site, defaultStr)
	}

	// Write weg.toml
	wegPath := filepath.Join(path, "weg.toml")
	if err := os.WriteFile(wegPath, []byte(wegContent), 0644); err != nil {
		return fmt.Errorf("failed to write weg.toml: %w", err)
	}

	// Create initial state
	st := state.NewState()
	st.Frappe.Version = frappeVersion
	st.Frappe.Database = "mariadb"

	for name, appInfo := range apps {
		st.AddApp(state.AppState{
			Name:   name,
			URL:    appInfo.URL,
			Branch: appInfo.Branch,
			Path:   appInfo.Path,
		})
	}

	for i, site := range sites {
		st.AddSite(state.SiteState{
			Name:        site,
			DefaultSite: i == 0,
		})
	}

	if err := st.Save(path); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	PrintInfo("Created weg.toml from existing bench")
	PrintInfo("Found %d apps and %d sites", len(apps), len(sites))
	PrintInfo("\nNext steps:")
	PrintInfo("  1. Review weg.toml and adjust as needed")
	PrintInfo("  2. Run 'weg status' to see current state")
	PrintInfo("  3. You can now use weg commands to manage this bench")

	return nil
}

// showExistingStatus shows status for already-initialized projects
func showExistingStatus(path string, result *config.DetectionResult) error {
	PrintInfo("Weg-managed project detected: %s", result.ContextDescription())

	if result.ConfigPath != "" {
		PrintInfo("Config: %s", result.ConfigPath)
	}

	// Load and show state
	st, err := state.Load(path)
	if err != nil {
		PrintVerbose("Could not load state: %v", err)
	} else if !st.IsEmpty() {
		PrintInfo("\nCurrent state:")
		PrintInfo("  Apps: %d installed", len(st.Apps))
		PrintInfo("  Sites: %d configured", len(st.Sites))
		if st.Frappe.Version != "" {
			PrintInfo("  Frappe: %s", st.Frappe.Version)
		}
	}

	PrintInfo("\n%s", result.SuggestAction())
	return nil
}

// Helper types and functions

type appInfo struct {
	URL    string
	Branch string
	Path   string
}

func scanAppsDirectory(appsDir string) (map[string]appInfo, error) {
	apps := make(map[string]appInfo)

	entries, err := os.ReadDir(appsDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		appPath := filepath.Join(appsDir, name)

		// Check for git remote
		url, branch := getGitInfo(appPath)

		apps[name] = appInfo{
			URL:    url,
			Branch: branch,
			Path:   appPath,
		}
	}

	return apps, nil
}

func scanSitesDirectory(sitesDir string) ([]string, error) {
	var sites []string

	entries, err := os.ReadDir(sitesDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Skip common non-site directories
		if name == "assets" || name == "common_site_config.json" {
			continue
		}

		// Check if it looks like a site (has site_config.json)
		configPath := filepath.Join(sitesDir, name, "site_config.json")
		if _, err := os.Stat(configPath); err == nil {
			sites = append(sites, name)
		}
	}

	return sites, nil
}

func getGitInfo(path string) (url, branch string) {
	// Try to get git remote URL
	gitDir := filepath.Join(path, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return "", ""
	}

	// Read git config for remote URL
	configPath := filepath.Join(gitDir, "config")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", ""
	}

	// Simple parsing for remote URL
	lines := strings.Split(string(data), "\n")
	inRemote := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[remote \"origin\"]") {
			inRemote = true
			continue
		}
		if inRemote && strings.HasPrefix(line, "url = ") {
			url = strings.TrimPrefix(line, "url = ")
		}
		if strings.HasPrefix(line, "[") && inRemote {
			break
		}
	}

	// Read HEAD for branch
	headPath := filepath.Join(gitDir, "HEAD")
	headData, err := os.ReadFile(headPath)
	if err == nil {
		head := strings.TrimSpace(string(headData))
		if strings.HasPrefix(head, "ref: refs/heads/") {
			branch = strings.TrimPrefix(head, "ref: refs/heads/")
		}
	}

	return url, branch
}

func detectFrappeVersion(frappePath string) string {
	// Try to detect from branch name
	_, branch := getGitInfo(frappePath)
	if branch != "" {
		if strings.HasPrefix(branch, "version-") {
			return strings.TrimPrefix(branch, "version-")
		}
		if branch == "develop" {
			return "develop"
		}
	}

	// Default to 15
	return "15"
}
