package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gavindsouza/weg/internal/apps"
	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/fsutil"
	"github.com/gavindsouza/weg/internal/services"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Synchronize environment with configuration",
	Long: `Apply configuration changes to the environment.

Sync compares the current configuration (weg.toml or pyproject.toml)
against the recorded state and applies any differences:

  - Installs new apps
  - Removes deleted apps
  - Updates changed app versions
  - Creates new sites
  - Removes deleted sites

By default, sync will show a preview of changes and ask for confirmation.

Examples:
  weg sync           # Interactive sync with confirmation
  weg sync -y        # Apply changes without confirmation
  weg sync --dry-run # Preview changes without applying`,
	RunE:         runSync,
	SilenceUsage: true, // Don't print usage on runtime errors
}

var (
	dryRun bool
)

func init() {
	rootCmd.AddCommand(syncCmd)
	syncCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without applying")
}

func runSync(cmd *cobra.Command, args []string) error {
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

	switch result.Context {
	case config.ContextWegApp:
		return syncApp(absPath, result)
	case config.ContextWegBench:
		return syncBench(absPath, result)
	case config.ContextFresh, config.ContextApp, config.ContextBench:
		return fmt.Errorf("not a weg-managed project. Run 'weg init' first")
	default:
		return fmt.Errorf("unknown context")
	}
}

func syncApp(path string, result *config.DetectionResult) error {
	PrintInfo("Syncing app-centric environment...")

	wegDir := filepath.Join(path, ".weg")
	wegTomlPath := filepath.Join(wegDir, "weg.toml")

	// For app-centric projects, use .weg/weg.toml for sync
	// This contains the full bench configuration
	if _, err := os.Stat(wegTomlPath); err == nil {
		// Use bench-style sync with .weg/weg.toml
		// ParseWegToml expects directory path, not file path
		benchConfig, err := config.ParseWegToml(wegDir)
		if err != nil {
			return fmt.Errorf("failed to parse .weg/weg.toml: %w", err)
		}

		// Validate config
		if err := config.ValidateBenchConfig(benchConfig); err != nil {
			return fmt.Errorf("invalid configuration: %w", err)
		}

		// Load state from project root (state module adds .weg internally)
		st, err := state.Load(path)
		if err != nil {
			st = state.NewState()
		}

		// Compute diff
		diff := state.ComputeDiffFromBenchConfig(benchConfig, st, wegDir)

		if diff.IsEmpty() {
			PrintInfo("Environment is up to date. Nothing to sync.")
			return nil
		}

		// Show changes
		showDiff(diff)

		if dryRun {
			PrintInfo("\nDry run - no changes applied.")
			return nil
		}

		// Confirm
		if !AssumeYes() {
			fmt.Print("\nApply these changes? [y/N]: ")
			reader := bufio.NewReader(os.Stdin)
			answer, _ := reader.ReadString('\n')
			answer = strings.TrimSpace(strings.ToLower(answer))
			if answer != "y" && answer != "yes" {
				PrintInfo("Cancelled.")
				return nil
			}
		}

		// Apply changes using .weg as the bench path
		if err := applyBenchChanges(wegDir, benchConfig, st, diff); err != nil {
			return fmt.Errorf("failed to apply changes: %w", err)
		}

		// Update state
		configHash, _ := state.ComputeConfigHash(wegTomlPath)
		st.UpdateConfigHash(configHash)
		st.Frappe.Version = benchConfig.Frappe.Version
		st.Frappe.Database = benchConfig.Frappe.Database

		if err := st.Save(path); err != nil {
			return fmt.Errorf("failed to save state: %w", err)
		}

		PrintInfo("\nSync complete!")
		return nil
	}

	// Fallback: Try pyproject.toml
	appConfig, err := config.ParsePyproject(path)
	if err != nil {
		return fmt.Errorf("failed to parse config: no .weg/weg.toml or pyproject.toml found")
	}

	// Validate config
	if err := config.ValidateAppConfig(appConfig); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Load state
	st, err := state.Load(path)
	if err != nil {
		st = state.NewState()
	}

	// Compute diff
	appName := filepath.Base(path)
	diff := state.ComputeDiffFromAppConfig(appConfig, appName, st)

	if diff.IsEmpty() {
		PrintInfo("Environment is up to date. Nothing to sync.")
		return nil
	}

	// Show changes
	showDiff(diff)

	if dryRun {
		PrintInfo("\nDry run - no changes applied.")
		return nil
	}

	// Confirm
	if !AssumeYes() {
		fmt.Print("\nApply these changes? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			PrintInfo("Cancelled.")
			return nil
		}
	}

	// Apply changes
	if err := applyAppChanges(path, appConfig, st, diff); err != nil {
		return fmt.Errorf("failed to apply changes: %w", err)
	}

	// Update state
	pyprojectPath := filepath.Join(path, "pyproject.toml")
	configHash, _ := state.ComputeConfigHash(pyprojectPath)
	st.UpdateConfigHash(configHash)
	st.Frappe.Version = appConfig.Dev.Frappe
	st.Frappe.Database = appConfig.Dev.Database

	if err := st.Save(path); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	PrintInfo("\nSync complete!")
	return nil
}

func syncBench(path string, result *config.DetectionResult) error {
	PrintInfo("Syncing bench environment...")

	// Parse config
	wegTomlPath := filepath.Join(path, "weg.toml")
	benchConfig, err := config.ParseWegToml(wegTomlPath)
	if err != nil {
		return fmt.Errorf("failed to parse weg.toml: %w", err)
	}

	// Validate config
	if err := config.ValidateBenchConfig(benchConfig); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Load state
	st, err := state.Load(path)
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	// Compute diff
	diff := state.ComputeDiffFromBenchConfig(benchConfig, st, path)

	if diff.IsEmpty() {
		PrintInfo("Environment is up to date. Nothing to sync.")
		return nil
	}

	// Show changes
	showDiff(diff)

	if dryRun {
		PrintInfo("\nDry run - no changes applied.")
		return nil
	}

	// Confirm
	if !AssumeYes() {
		fmt.Print("\nApply these changes? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			PrintInfo("Cancelled.")
			return nil
		}
	}

	// Apply changes
	if err := applyBenchChanges(path, benchConfig, st, diff); err != nil {
		return fmt.Errorf("failed to apply changes: %w", err)
	}

	// Update state
	configHash, _ := state.ComputeConfigHash(filepath.Join(path, "weg.toml"))
	st.UpdateConfigHash(configHash)
	st.Frappe.Version = benchConfig.Frappe.Version
	st.Frappe.Database = benchConfig.Frappe.Database

	if err := st.Save(path); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	PrintInfo("\nSync complete!")
	return nil
}

func showDiff(diff *state.Diff) {
	fmt.Printf("\nChanges to apply (%d total):\n\n", diff.TotalChanges())

	if len(diff.AppsToAdd) > 0 {
		fmt.Println("Apps to install:")
		for _, app := range diff.AppsToAdd {
			fmt.Printf("  + %s\n", app)
		}
	}

	if len(diff.AppsToRemove) > 0 {
		fmt.Println("Apps to remove:")
		for _, app := range diff.AppsToRemove {
			fmt.Printf("  - %s\n", app)
		}
	}

	if len(diff.AppsToUpdate) > 0 {
		fmt.Println("Apps to update:")
		for _, update := range diff.AppsToUpdate {
			if update.NewBranch != "" {
				fmt.Printf("  ~ %s: %s -> %s\n", update.Name, update.OldBranch, update.NewBranch)
			}
			if update.NewURL != "" {
				fmt.Printf("  ~ %s: URL changed\n", update.Name)
			}
			if update.DepsChanged {
				fmt.Printf("  ~ %s: dependencies changed (will reinstall)\n", update.Name)
			}
		}
	}

	if len(diff.SitesToAdd) > 0 {
		fmt.Println("Sites to create:")
		for _, site := range diff.SitesToAdd {
			fmt.Printf("  + %s\n", site)
		}
	}

	if len(diff.SitesToRemove) > 0 {
		fmt.Println("Sites to remove:")
		for _, site := range diff.SitesToRemove {
			fmt.Printf("  - %s\n", site)
		}
	}

	if len(diff.SitesToUpdate) > 0 {
		fmt.Println("Sites to update:")
		for _, update := range diff.SitesToUpdate {
			fmt.Printf("  ~ %s\n", update.Name)
			for _, app := range update.AppsToAdd {
				fmt.Printf("      + install %s\n", app)
			}
			for _, app := range update.AppsToRemove {
				fmt.Printf("      - uninstall %s\n", app)
			}
		}
	}

	if diff.FrappeChanged {
		fmt.Println("Frappe settings changed (manual update may be required)")
	}

	if diff.ServicesChanged {
		fmt.Println("Services configuration changed:")
		if diff.NewServices.WebPort != 0 {
			fmt.Printf("  ~ web port: %d\n", diff.NewServices.WebPort)
		}
		if diff.NewServices.SocketPort != 0 {
			fmt.Printf("  ~ socket port: %d\n", diff.NewServices.SocketPort)
		}
		if len(diff.NewServices.Workers) > 0 {
			fmt.Printf("  ~ workers: %v\n", diff.NewServices.Workers)
		}
	}
}

func applyAppChanges(path string, cfg *config.AppConfig, st *state.State, diff *state.Diff) error {
	// For app-centric, we need to create/update the .weg directory
	wegDir := filepath.Join(path, ".weg")

	// Ensure the environment is initialized before installing apps
	if err := ensureEnvironment(wegDir, cfg.Dev.Frappe); err != nil {
		return fmt.Errorf("failed to initialize environment: %w", err)
	}

	// Install new apps
	for _, appName := range diff.AppsToAdd {
		PrintInfo("Installing %s...", appName)

		if appName == "frappe" {
			// Install Frappe framework
			if err := installFrappe(wegDir, cfg.Dev.Frappe); err != nil {
				return fmt.Errorf("failed to install frappe: %w", err)
			}
		} else if appName == filepath.Base(path) {
			// Install the main app (the current directory)
			if err := installLocalApp(wegDir, path); err != nil {
				return fmt.Errorf("failed to install app: %w", err)
			}
		} else {
			// Install dependency app
			for _, dep := range cfg.Dependencies.Apps {
				if dep.Name == appName {
					if err := installApp(wegDir, dep.Name, dep.URL, dep.Branch); err != nil {
						return fmt.Errorf("failed to install %s: %w", appName, err)
					}
					break
				}
			}
		}

		appPath := filepath.Join(wegDir, "apps", appName)
		st.AddApp(state.AppState{
			Name:          appName,
			PyprojectHash: state.ComputePyprojectHash(appPath),
		})
	}

	// Remove apps
	for _, appName := range diff.AppsToRemove {
		PrintInfo("Removing %s...", appName)
		if err := removeApp(wegDir, appName); err != nil {
			PrintVerbose("Warning: failed to remove %s: %v", appName, err)
		}
		st.RemoveApp(appName)
	}

	// Update apps.txt - required for bench to recognize apps
	if err := updateAppsTxt(wegDir, st); err != nil {
		PrintVerbose("Warning: failed to update apps.txt: %v", err)
	}

	// Setup asset symlinks for static files
	if err := setupAssets(wegDir); err != nil {
		PrintVerbose("Warning: failed to setup assets: %v", err)
	}

	// Ensure common_site_config.json exists with redis config
	// For pyproject.toml based projects, use defaults (nil config)
	if err := ensureCommonSiteConfig(wegDir, nil); err != nil {
		PrintVerbose("Warning: failed to create common_site_config.json: %v", err)
	}

	// Create sites
	sitesDir := filepath.Join(wegDir, "sites")
	for _, siteName := range diff.SitesToAdd {
		PrintInfo("Creating site %s...", siteName)

		siteCfg := &config.SiteConfig{
			Name:        siteName,
			DefaultSite: true, // First site is default for app-centric
		}

		// Get list of all installed apps to install on the site
		var appsToInstall []string
		for appName := range st.Apps {
			appsToInstall = append(appsToInstall, appName)
		}

		if err := createSite(sitesDir, siteCfg, appsToInstall); err != nil {
			return fmt.Errorf("failed to create site %s: %w", siteName, err)
		}

		st.AddSite(state.SiteState{
			Name:        siteName,
			DefaultSite: siteCfg.DefaultSite,
		})
	}

	return nil
}

func applyBenchChanges(path string, cfg *config.BenchConfig, st *state.State, diff *state.Diff) error {
	appsDir := filepath.Join(path, "apps")

	// Install new apps
	for _, appName := range diff.AppsToAdd {
		PrintInfo("Installing %s...", appName)

		appCfg, ok := cfg.Apps[appName]
		if !ok {
			continue
		}

		if appCfg.Path != "" {
			// Local app - symlink or copy
			if err := linkLocalApp(appsDir, appName, appCfg.Path); err != nil {
				return fmt.Errorf("failed to link %s: %w", appName, err)
			}
		} else {
			// Remote app - clone
			if err := installApp(appsDir, appName, appCfg.URL, appCfg.Branch); err != nil {
				return fmt.Errorf("failed to install %s: %w", appName, err)
			}
		}

		appPath := filepath.Join(appsDir, appName)
		st.AddApp(state.AppState{
			Name:          appName,
			URL:           appCfg.URL,
			Branch:        appCfg.Branch,
			Path:          appCfg.Path,
			PyprojectHash: state.ComputePyprojectHash(appPath),
		})
	}

	// Remove apps
	for _, appName := range diff.AppsToRemove {
		PrintInfo("Removing %s...", appName)
		if err := removeApp(appsDir, appName); err != nil {
			PrintVerbose("Warning: failed to remove %s: %v", appName, err)
		}
		st.RemoveApp(appName)
	}

	// Update apps
	for _, update := range diff.AppsToUpdate {
		PrintInfo("Updating %s...", update.Name)
		appPath := filepath.Join(appsDir, update.Name)

		if update.NewBranch != "" {
			if err := checkoutBranch(appPath, update.NewBranch); err != nil {
				return fmt.Errorf("failed to update %s: %w", update.Name, err)
			}
		}

		// Reinstall Python deps if pyproject.toml changed
		if update.DepsChanged {
			PrintInfo("  Reinstalling dependencies for %s...", update.Name)
			opts := apps.InstallOptions{
				BenchPath: path,
				AppsDir:   appsDir,
				Verbose:   IsVerbose(),
			}
			if err := apps.InstallPythonDeps(appPath, opts); err != nil {
				return fmt.Errorf("failed to reinstall deps for %s: %w", update.Name, err)
			}
		}

		// Preserve existing state fields and update changed ones
		appState := st.Apps[update.Name]
		if update.NewURL != "" {
			appState.URL = update.NewURL
		}
		if update.NewBranch != "" {
			appState.Branch = update.NewBranch
		}
		appState.PyprojectHash = state.ComputePyprojectHash(appPath)
		st.Apps[update.Name] = appState
	}

	// Update apps.txt - required for bench to recognize apps
	// Must be done BEFORE site creation since new-site needs apps.txt
	if err := updateAppsTxt(path, st); err != nil {
		return fmt.Errorf("failed to update apps.txt: %w", err)
	}

	// Setup asset symlinks for static files
	if err := setupAssets(path); err != nil {
		PrintVerbose("Warning: failed to setup assets: %v", err)
	}

	// Ensure common_site_config.json exists with settings from weg.toml
	if err := ensureCommonSiteConfig(path, cfg); err != nil {
		PrintVerbose("Warning: failed to create common_site_config.json: %v", err)
	}

	// Handle sites
	sitesDir := filepath.Join(path, "sites")

	for _, siteName := range diff.SitesToAdd {
		PrintInfo("Creating site %s...", siteName)

		// Find site config
		var siteCfg *config.SiteConfig
		for i := range cfg.Sites {
			if cfg.Sites[i].Name == siteName {
				siteCfg = &cfg.Sites[i]
				break
			}
		}

		if siteCfg != nil {
			if err := createSite(sitesDir, siteCfg, siteCfg.Apps); err != nil {
				return fmt.Errorf("failed to create site %s: %w", siteName, err)
			}
		}

		st.AddSite(state.SiteState{
			Name:        siteName,
			DefaultSite: siteCfg != nil && siteCfg.DefaultSite,
		})
	}

	for _, siteName := range diff.SitesToRemove {
		PrintInfo("Removing site %s...", siteName)
		if err := removeSite(sitesDir, siteName); err != nil {
			PrintVerbose("Warning: failed to remove site %s: %v", siteName, err)
		}
		st.RemoveSite(siteName)
	}

	// Update sites (install/uninstall apps)
	for _, update := range diff.SitesToUpdate {
		for _, appName := range update.AppsToAdd {
			PrintInfo("Installing %s on site %s...", appName, update.Name)
			if err := installAppOnSite(path, update.Name, appName); err != nil {
				return fmt.Errorf("failed to install %s on site %s: %w", appName, update.Name, err)
			}

			// Update site state
			for name, site := range st.Sites {
				if name == update.Name {
					site.Apps = append(site.Apps, appName)
					st.Sites[name] = site
					break
				}
			}
		}

		for _, appName := range update.AppsToRemove {
			PrintInfo("Uninstalling %s from site %s...", appName, update.Name)
			if err := uninstallAppFromSite(path, update.Name, appName); err != nil {
				PrintVerbose("Warning: failed to uninstall %s from %s: %v", appName, update.Name, err)
			}

			// Update site state
			for name, site := range st.Sites {
				if name == update.Name {
					for j, a := range site.Apps {
						if a == appName {
							site.Apps = append(site.Apps[:j], site.Apps[j+1:]...)
							st.Sites[name] = site
							break
						}
					}
					break
				}
			}
		}
	}

	// Regenerate process-compose.yaml if services changed
	if diff.ServicesChanged {
		PrintInfo("Updating services configuration...")
		if err := regenerateProcessCompose(path, cfg); err != nil {
			return fmt.Errorf("failed to update process-compose.yaml: %w", err)
		}
		st.Services = diff.NewServices
	}

	return nil
}

// updateAppsTxt writes the apps.txt file with installed apps
// This file is required by frappe/bench to recognize which apps are installed
// Note: apps.txt goes in sites/ directory (frappe uses sites_path=".")
// App names must use underscores (Python module format)
func updateAppsTxt(benchPath string, st *state.State) error {
	sitesDir := filepath.Join(benchPath, "sites")
	if err := os.MkdirAll(sitesDir, 0755); err != nil {
		return fmt.Errorf("failed to create sites directory: %w", err)
	}
	appsTxtPath := filepath.Join(sitesDir, "apps.txt")

	// Get app names in order (frappe first)
	var apps []string
	hasFramework := false
	for name := range st.Apps {
		// Convert to Python module name (hyphens to underscores)
		moduleName := strings.ReplaceAll(name, "-", "_")
		if moduleName == "frappe" {
			hasFramework = true
		} else {
			apps = append(apps, moduleName)
		}
	}

	// Sort and prepend frappe
	var orderedApps []string
	if hasFramework {
		orderedApps = append(orderedApps, "frappe")
	}
	orderedApps = append(orderedApps, apps...)

	// Write apps.txt atomically
	content := strings.Join(orderedApps, "\n") + "\n"
	return fsutil.AtomicWriteString(appsTxtPath, content, 0644)
}

// setupAssets creates symlinks for app assets in sites/assets/
// This is required for Frappe to serve static files (images, css, js, etc.)
func setupAssets(benchPath string) error {
	appsDir := filepath.Join(benchPath, "apps")
	assetsDir := filepath.Join(benchPath, "sites", "assets")

	// Ensure assets directory exists
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		return fmt.Errorf("failed to create assets directory: %w", err)
	}

	// Read apps directory
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		return fmt.Errorf("failed to read apps directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		appName := entry.Name()
		// App's public directory: apps/{app}/{app}/public/
		publicDir := filepath.Join(appsDir, appName, appName, "public")

		if _, err := os.Stat(publicDir); os.IsNotExist(err) {
			// Try without nested directory (some apps have public at root)
			publicDir = filepath.Join(appsDir, appName, "public")
			if _, err := os.Stat(publicDir); os.IsNotExist(err) {
				continue // No public directory
			}
		}

		// Create symlink: sites/assets/{app} -> apps/{app}/{app}/public/
		assetLink := filepath.Join(assetsDir, appName)

		// Remove existing link/directory if it exists
		if info, err := os.Lstat(assetLink); err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				// It's a symlink, check if it points to the right place
				target, _ := os.Readlink(assetLink)
				if target == publicDir {
					continue // Already correct
				}
			}
			os.RemoveAll(assetLink)
		}

		// Create relative symlink
		relPath, err := filepath.Rel(assetsDir, publicDir)
		if err != nil {
			relPath = publicDir // Fall back to absolute
		}

		if err := os.Symlink(relPath, assetLink); err != nil {
			PrintVerbose("Warning: failed to create asset symlink for %s: %v", appName, err)
		}
	}

	return nil
}

// ensureCommonSiteConfig creates common_site_config.json from weg.toml config
// If benchConfig is nil, uses defaults
func ensureCommonSiteConfig(benchPath string, benchConfig *config.BenchConfig) error {
	sitesDir := filepath.Join(benchPath, "sites")
	configPath := filepath.Join(sitesDir, "common_site_config.json")

	var cfg map[string]interface{}

	if benchConfig != nil {
		// Generate config from weg.toml settings
		cfg = benchConfig.GenerateCommonSiteConfig(nil)
	} else {
		// Use defaults
		cfg = map[string]interface{}{
			"redis_cache":    "redis://localhost:6379/0",
			"redis_queue":    "redis://localhost:6379/1",
			"redis_socketio": "redis://localhost:6379/2",
			"webserver_port": 8000,
			"socketio_port":  9000,
			"developer_mode": 1,
		}
	}

	// Write config atomically
	data, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return err
	}

	return fsutil.AtomicWrite(configPath, data, 0644)
}

// Real implementations using internal/apps package

func installFrappe(wegDir, version string) error {
	appsDir := filepath.Join(wegDir, "apps")
	if err := os.MkdirAll(appsDir, 0755); err != nil {
		return fmt.Errorf("failed to create apps directory: %w", err)
	}

	opts := apps.InstallOptions{
		BenchPath:     wegDir,
		AppsDir:       appsDir,
		FrappeVersion: version,
		Verbose:       IsVerbose(),
	}

	branch := "version-" + version
	if version == "develop" {
		branch = "develop"
	}

	return apps.InstallApp("frappe", "https://github.com/frappe/frappe", branch, opts)
}

func installLocalApp(wegDir, appPath string) error {
	appsDir := filepath.Join(wegDir, "apps")
	appName := filepath.Base(appPath)

	opts := apps.InstallOptions{
		BenchPath: wegDir,
		AppsDir:   appsDir,
		Verbose:   IsVerbose(),
	}

	return apps.LinkLocalApp(appName, appPath, opts)
}

func installApp(appsDir, name, url, branch string) error {
	benchPath := filepath.Dir(appsDir)
	opts := apps.InstallOptions{
		BenchPath: benchPath,
		AppsDir:   appsDir,
		Verbose:   IsVerbose(),
	}

	return apps.InstallApp(name, url, branch, opts)
}

func linkLocalApp(appsDir, name, localPath string) error {
	benchPath := filepath.Dir(appsDir)

	// Resolve relative paths from bench directory, not CWD
	if !filepath.IsAbs(localPath) {
		localPath = filepath.Join(benchPath, localPath)
	}

	opts := apps.InstallOptions{
		BenchPath: benchPath,
		AppsDir:   appsDir,
		Verbose:   IsVerbose(),
	}

	return apps.LinkLocalApp(name, localPath, opts)
}

func removeApp(appsDir, name string) error {
	benchPath := filepath.Dir(appsDir)
	opts := apps.InstallOptions{
		BenchPath: benchPath,
		AppsDir:   appsDir,
		Verbose:   IsVerbose(),
	}

	return apps.RemoveApp(name, opts)
}

func checkoutBranch(appPath, branch string) error {
	return apps.Checkout(appPath, branch)
}

func createSite(sitesDir string, cfg *config.SiteConfig, appsToInstall []string) error {
	benchPath := filepath.Dir(sitesDir)
	mysqlSocket := filepath.Join(benchPath, ".devbox/virtenv/mariadb/run/mysql.sock")

	// Start mariadb service first - stop it first if it's in a bad state
	PrintVerbose("Starting MariaDB...")

	// If socket doesn't exist but service claims to be running, restart it
	if _, err := os.Stat(mysqlSocket); os.IsNotExist(err) {
		PrintVerbose("MariaDB socket not found, restarting service...")
		runCmdInDir(benchPath, "devbox", "services", "stop")
	}

	if err := runCmdInDir(benchPath, "devbox", "services", "start", "mariadb"); err != nil {
		PrintVerbose("Warning: could not start mariadb: %v", err)
	}

	// Wait for mariadb socket to be available (up to 30 seconds)
	PrintVerbose("Waiting for MariaDB to be ready...")
	socketFound := false
	for i := 0; i < 30; i++ {
		if _, err := os.Stat(mysqlSocket); err == nil {
			socketFound = true
			break
		}
		time.Sleep(1 * time.Second)
	}

	if !socketFound {
		return fmt.Errorf("MariaDB socket not found at %s after 30 seconds. Try running 'devbox services stop' and then 'weg sync' again", mysqlSocket)
	}

	// Build install-app flags for non-frappe apps
	var installAppFlags string
	for _, app := range appsToInstall {
		if app != "frappe" {
			// Convert to module name (hyphens to underscores)
			moduleName := strings.ReplaceAll(app, "-", "_")
			installAppFlags += fmt.Sprintf(" --install-app=%s", moduleName)
		}
	}

	// Create the site using bench new-site via the venv Python
	// Use the devbox mariadb socket and empty root password
	// NOTE: frappe commands must run from the sites directory (where apps.txt is)
	PrintVerbose("Running new-site for %s...", cfg.Name)
	shellCmd := fmt.Sprintf(
		`cd %s && ../.venv/bin/python -m frappe.utils.bench_helper frappe new-site %s --admin-password=admin --db-root-password= --db-socket=%s%s`,
		sitesDir, cfg.Name, mysqlSocket, installAppFlags,
	)

	if err := runCmdInDir(benchPath, "devbox", "run", "--", "sh", "-c", shellCmd); err != nil {
		return fmt.Errorf("failed to create site: %w", err)
	}

	// Note: currentsite.txt is deprecated in Frappe v15+
	// Users should pass --site explicitly or use 'bench use <site>'

	return nil
}

func removeSite(sitesDir, name string) error {
	sitePath := filepath.Join(sitesDir, name)
	benchPath := filepath.Dir(sitesDir)

	PrintVerbose("Removing site: %s", sitePath)

	// Try to drop database using bench drop-site first
	// This properly removes the database before removing the directory
	dropArgs := []string{"drop-site", name, "--force", "--no-backup"}
	dropCmd := exec.Command("bench", dropArgs...)
	dropCmd.Dir = benchPath

	if err := dropCmd.Run(); err != nil {
		// bench drop-site failed (maybe bench not available or database already gone)
		// Fall back to just removing the directory
		PrintVerbose("bench drop-site failed: %v, removing directory only", err)
	}

	return os.RemoveAll(sitePath)
}

func installAppOnSite(benchPath, siteName, appName string) error {
	opts := apps.InstallOptions{
		BenchPath: benchPath,
		AppsDir:   filepath.Join(benchPath, "apps"),
		Verbose:   IsVerbose(),
	}

	return apps.InstallAppOnSite(siteName, appName, opts)
}

func uninstallAppFromSite(benchPath, siteName, appName string) error {
	opts := apps.InstallOptions{
		BenchPath: benchPath,
		AppsDir:   filepath.Join(benchPath, "apps"),
		Verbose:   IsVerbose(),
	}

	return apps.UninstallAppFromSite(siteName, appName, opts)
}

// ensureEnvironment ensures the .weg directory has devbox and venv set up
func ensureEnvironment(wegDir, frappeVersion string) error {
	// Create required directories
	dirs := []string{
		wegDir,
		filepath.Join(wegDir, "apps"),
		filepath.Join(wegDir, "sites"),
		filepath.Join(wegDir, "config"),
		filepath.Join(wegDir, "logs"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Check if devbox is initialized
	devboxJSON := filepath.Join(wegDir, "devbox.json")
	if _, err := os.Stat(devboxJSON); os.IsNotExist(err) {
		PrintInfo("Initializing development environment...")

		// Create .envrc
		envrc := `VENV_DIR=$PWD/env
export UV_PYTHON=$PWD/.devbox/nix/profile/default/bin/python
export UV_VENV_PATH=$VENV_DIR
export VIRTUAL_ENV=$VENV_DIR
eval "$(devbox generate direnv --print-envrc -e VENV_DIR=$VENV_DIR -e UV_PYTHON=$UV_PYTHON -e UV_VENV_PATH=$UV_VENV_PATH -e VIRTUAL_ENV=$VIRTUAL_ENV)"
`
		if err := os.WriteFile(filepath.Join(wegDir, ".envrc"), []byte(envrc), 0644); err != nil {
			return fmt.Errorf("failed to write .envrc: %w", err)
		}

		// Initialize devbox
		PrintVerbose("Initializing devbox...")
		if err := runCmdInDir(wegDir, "devbox", "init"); err != nil {
			return fmt.Errorf("devbox init failed: %w", err)
		}

		// Add required packages via devbox
		PrintVerbose("Installing dependencies via devbox...")
		packages := getDevboxPackages(frappeVersion)
		args := append([]string{"add"}, packages...)
		if err := runCmdInDir(wegDir, "devbox", args...); err != nil {
			return fmt.Errorf("devbox add failed: %w", err)
		}
	}

	// Devbox's Python plugin automatically creates .venv, no need to create env/

	return nil
}

// regenerateProcessCompose regenerates process-compose.yaml from config
func regenerateProcessCompose(benchPath string, cfg *config.BenchConfig) error {
	opts := services.ComposeOptions{
		BenchPath:     benchPath,
		WebPort:       cfg.Services.Web.Port,
		SocketPort:    cfg.Services.Web.SocketPort,
		IncludeRedis:  false, // Devbox manages redis
		IncludeWatch:  true,
		UseVenvPython: true,
		Workers:       cfg.Services.Workers,
	}

	// Set defaults
	if opts.WebPort == 0 {
		opts.WebPort = 8000
	}
	if opts.SocketPort == 0 {
		opts.SocketPort = 9000
	}

	return services.GenerateAndWrite(opts)
}
