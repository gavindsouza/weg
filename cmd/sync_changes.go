package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gavindsouza/weg/internal/apps"
	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/state"
)

// applyAppChanges applies configuration changes to an app-centric project
func applyAppChanges(path string, cfg *config.AppConfig, st *state.State, diff *state.Diff) error {
	// For app-centric, we need to create/update the .weg directory
	wegDir := filepath.Join(path, ".weg")

	// Ensure the environment is initialized before installing apps
	if err := ensureEnvironment(wegDir, cfg.Dev.Frappe); err != nil {
		return fmt.Errorf("failed to initialize environment: %w", err)
	}

	// Install new apps
	if err := installAppCentricApps(wegDir, path, cfg, st, diff); err != nil {
		return err
	}

	// Remove apps
	if err := removeApps(wegDir, st, diff.AppsToRemove); err != nil {
		return err
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
	if err := createAppCentricSites(wegDir, st, diff.SitesToAdd); err != nil {
		return err
	}

	return nil
}

// installAppCentricApps installs new apps for an app-centric project
func installAppCentricApps(wegDir, appPath string, cfg *config.AppConfig, st *state.State, diff *state.Diff) error {
	for _, appName := range diff.AppsToAdd {
		PrintInfo("Installing %s...", appName)

		if appName == "frappe" {
			// Install Frappe framework
			if err := installFrappe(wegDir, cfg.Dev.Frappe); err != nil {
				return fmt.Errorf("failed to install frappe: %w", err)
			}
		} else if appName == filepath.Base(appPath) {
			// Install the main app (the current directory)
			if err := installLocalApp(wegDir, appPath); err != nil {
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

		appsDir := filepath.Join(wegDir, "apps")
		appStatePath := filepath.Join(appsDir, appName)
		st.AddApp(state.AppState{
			Name:          appName,
			PyprojectHash: state.ComputePyprojectHash(appStatePath),
		})
	}
	return nil
}

// createAppCentricSites creates new sites for an app-centric project
func createAppCentricSites(wegDir string, st *state.State, sitesToAdd []string) error {
	sitesDir := filepath.Join(wegDir, "sites")
	for _, siteName := range sitesToAdd {
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

// applyBenchChanges applies configuration changes to a bench-centric project
func applyBenchChanges(path string, cfg *config.BenchConfig, st *state.State, diff *state.Diff) error {
	appsDir := filepath.Join(path, "apps")

	// Install new apps
	if err := installBenchApps(appsDir, path, cfg, st, diff.AppsToAdd); err != nil {
		return err
	}

	// Remove apps
	if err := removeApps(appsDir, st, diff.AppsToRemove); err != nil {
		return err
	}

	// Update apps
	if err := updateBenchApps(appsDir, path, st, diff.AppsToUpdate); err != nil {
		return err
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
	if err := handleBenchSites(path, cfg, st, diff); err != nil {
		return err
	}

	// Regenerate process-compose.yaml if services changed
	if diff.ServicesChanged {
		PrintInfo("Updating services configuration...")
		if err := regenerateProcessCompose(path, cfg); err != nil {
			return fmt.Errorf("failed to update process-compose.yaml: %w", err)
		}
		st.Services = diff.NewServices
	}

	// Sync app-defined services (packages and processes from [tool.weg.services])
	PrintInfo("Syncing app services...")
	if err := syncAppServices(path); err != nil {
		PrintVerbose("Warning: failed to sync app services: %v", err)
	}

	return nil
}

// installBenchApps installs new apps for a bench-centric project
func installBenchApps(appsDir, benchPath string, cfg *config.BenchConfig, st *state.State, appsToAdd []string) error {
	for _, appName := range appsToAdd {
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
	return nil
}

// removeApps removes apps from the bench/app environment
func removeApps(appsDir string, st *state.State, appsToRemove []string) error {
	for _, appName := range appsToRemove {
		PrintInfo("Removing %s...", appName)
		if err := removeApp(appsDir, appName); err != nil {
			PrintVerbose("Warning: failed to remove %s: %v", appName, err)
		}
		st.RemoveApp(appName)
	}
	return nil
}

// updateBenchApps updates existing apps (branch changes, dependency reinstalls)
func updateBenchApps(appsDir, benchPath string, st *state.State, appsToUpdate []state.AppUpdate) error {
	for _, update := range appsToUpdate {
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
				BenchPath: benchPath,
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
	return nil
}

// handleBenchSites handles site creation, removal, and updates for bench projects
func handleBenchSites(benchPath string, cfg *config.BenchConfig, st *state.State, diff *state.Diff) error {
	sitesDir := filepath.Join(benchPath, "sites")

	// Create new sites
	if err := createBenchSites(sitesDir, cfg, st, diff.SitesToAdd); err != nil {
		return err
	}

	// Remove sites
	if err := removeBenchSites(sitesDir, st, diff.SitesToRemove); err != nil {
		return err
	}

	// Update sites (install/uninstall apps)
	if err := updateBenchSites(benchPath, st, diff.SitesToUpdate); err != nil {
		return err
	}

	return nil
}

// createBenchSites creates new sites for bench projects
func createBenchSites(sitesDir string, cfg *config.BenchConfig, st *state.State, sitesToAdd []string) error {
	for _, siteName := range sitesToAdd {
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
	return nil
}

// removeBenchSites removes sites from bench projects
func removeBenchSites(sitesDir string, st *state.State, sitesToRemove []string) error {
	for _, siteName := range sitesToRemove {
		PrintInfo("Removing site %s...", siteName)
		if err := removeSite(sitesDir, siteName); err != nil {
			PrintVerbose("Warning: failed to remove site %s: %v", siteName, err)
		}
		st.RemoveSite(siteName)
	}
	return nil
}

// updateBenchSites updates sites by installing/uninstalling apps
func updateBenchSites(benchPath string, st *state.State, sitesToUpdate []state.SiteUpdate) error {
	for _, update := range sitesToUpdate {
		// Install apps on site
		for _, appName := range update.AppsToAdd {
			PrintInfo("Installing %s on site %s...", appName, update.Name)
			if err := installAppOnSite(benchPath, update.Name, appName); err != nil {
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

		// Uninstall apps from site
		for _, appName := range update.AppsToRemove {
			PrintInfo("Uninstalling %s from site %s...", appName, update.Name)
			if err := uninstallAppFromSite(benchPath, update.Name, appName); err != nil {
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
	return nil
}

// App installation and management functions

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

// Site management functions

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
		`cd %s && ../env/bin/python -m frappe.utils.bench_helper frappe new-site %s --admin-password=admin --db-root-password= --db-socket=%s%s`,
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

	// Devbox's Python plugin automatically creates env, no need to create env/

	return nil
}
