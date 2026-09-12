package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/apps"
	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/fsutil"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/services"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/gavindsouza/weg/tools"
)

// applyAppChanges applies configuration changes to an app-centric project
func applyAppChanges(path string, cfg *config.AppConfig, st *state.State, diff *state.Diff) error {
	// For app-centric, we need to create/update the .weg directory
	wegDir := filepath.Join(path, ".weg")

	// Ensure the environment is initialized before installing apps
	if err := ensureEnvironment(wegDir, cfg.Dev.Frappe, ""); err != nil {
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
		output.Warningf("failed to write common_site_config.json: %v", err)
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
			Apps:        appsToInstall,
			DefaultSite: siteCfg.DefaultSite,
		})
	}
	return nil
}

// applyBenchChanges applies configuration changes to a bench-centric project
func applyBenchChanges(path string, cfg *config.BenchConfig, st *state.State, diff *state.Diff) error {
	appsDir := filepath.Join(path, "apps")

	if err := ensureEnvironment(path, cfg.Frappe.Version, cfg.Services.Database.Version); err != nil {
		return fmt.Errorf("failed to initialize environment: %w", err)
	}
	if err := syncDevboxEnv(path, cfg.Env); err != nil {
		return fmt.Errorf("failed to apply [env] to devbox.json: %w", err)
	}

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
		output.Warningf("failed to write common_site_config.json: %v", err)
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
		output.Warningf("failed to sync app services: %v", err)
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

		// Record the apps: without them the next sync sees every app as
		// missing and installs them all again.
		var apps []string
		if siteCfg != nil {
			apps = siteCfg.Apps
		}
		st.AddSite(state.SiteState{
			Name:        siteName,
			Apps:        apps,
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
			output.Warningf("failed to remove site %s (kept in state, the next sync retries): %v", siteName, err)
			continue
		}
		st.RemoveSite(siteName)
	}
	return nil
}

// updateBenchSites updates sites by installing/uninstalling apps
func updateBenchSites(benchPath string, st *state.State, sitesToUpdate []state.SiteUpdate) error {
	if len(sitesToUpdate) > 0 {
		if _, err := services.StartDatabase(benchPath); err != nil {
			return err
		}
	}
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
	// MariaDB, and redis: new-site talks to the redis queue.
	PrintVerbose("Starting MariaDB and Redis...")
	mysqlSocket, err := services.StartDatabase(benchPath)
	if err != nil {
		return err
	}

	// Ensure the devbox MariaDB root has a known password. New installs start
	// passwordless (socket-only, empty root); the site lives inside .weg so we
	// can set the password with the socket root connection before creating it.
	rootPassword := ensureMariaDBRootPassword(benchPath, mysqlSocket)

	// Build command arguments safely (no shell interpolation)
	adminPass := cfg.AdminPass
	if adminPass == "" {
		adminPass = "admin"
	}
	// new-site's database credentials are not the config's to replace;
	// checked before anything is created.
	for key := range cfg.Config {
		if strings.HasPrefix(key, "db_") {
			return fmt.Errorf("site %s: config key %q is set by new-site and can't be overridden", cfg.Name, key)
		}
	}

	// A site left behind by a failed app install is resumed rather than
	// re-created: new-site would refuse, and install-app skips installed apps.
	siteConfig := filepath.Join(sitesDir, cfg.Name, "site_config.json")
	resumed := fsutil.FileExists(siteConfig)
	if resumed {
		PrintVerbose("Site %s exists, resuming its config and app installs...", cfg.Name)
	} else {
		// Through devbox: new-site shells out to the mariadb client to load
		// the schema, and that client only exists on devbox's PATH.
		PrintVerbose("Running new-site for %s...", cfg.Name)
		if err := runFrappe(benchPath, "frappe", "new-site", cfg.Name,
			"--admin-password="+adminPass,
			"--db-root-username=root",
			"--db-root-password="+rootPassword,
			"--db-socket="+mysqlSocket,
		); err != nil {
			return fmt.Errorf("failed to create site: %w", err)
		}
	}

	// The site's [sites] config goes in before the apps: an app's
	// before_install may refuse unless a flag is already set.
	if len(cfg.Config) > 0 {
		if err := mergeConfigJSON(siteConfig, cfg.Config); err != nil {
			return fmt.Errorf("failed to apply config for %s: %w", cfg.Name, err)
		}
	}

	var apps []string
	for _, app := range appsToInstall {
		if app != "frappe" {
			apps = append(apps, strings.ReplaceAll(app, "-", "_"))
		}
	}
	if len(apps) > 0 {
		PrintVerbose("Installing apps on %s...", cfg.Name)
		if err := runFrappe(benchPath, append([]string{"frappe", "--site", cfg.Name, "install-app"}, apps...)...); err != nil {
			if resumed {
				return fmt.Errorf("failed to install apps on existing site %s (if its database is gone, remove %s and re-run): %w", cfg.Name, filepath.Join(sitesDir, cfg.Name), err)
			}
			return fmt.Errorf("failed to install apps on %s: %w", cfg.Name, err)
		}
	}

	// Note: currentsite.txt is deprecated in Frappe v15+
	// Users should pass --site explicitly or use 'bench use <site>'

	return nil
}

// runFrappe runs a frappe bench_helper command with output attached.
func runFrappe(benchPath string, args ...string) error {
	cmd := tools.FrappeCmd(benchPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ensureMariaDBRootPassword makes sure the devbox-managed MariaDB root login
// has the bench's root password (config.DBRootPassword). Fresh devbox installs
// start passwordless (socket-only, empty root). Returns the password to pass
// to frappe new-site.
func ensureMariaDBRootPassword(benchPath, mysqlSocket string) string {
	rootPassword := config.DBRootPassword(benchPath)
	// Try the known password first (idempotent).
	if services.MariaDBRoot(benchPath, mysqlSocket, rootPassword, "SELECT 1") == nil {
		return rootPassword
	}
	// Passwordless root (fresh devbox install): set it now.
	escaped := strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(rootPassword)
	stmt := "ALTER USER 'root'@'localhost' IDENTIFIED BY '" + escaped + "'; FLUSH PRIVILEGES;"
	if err := services.MariaDBRoot(benchPath, mysqlSocket, "", stmt); err != nil {
		PrintVerbose("Warning: could not set MariaDB root password: %v", err)
	}
	return rootPassword
}

func removeSite(sitesDir, name string) error {
	benchPath := filepath.Dir(sitesDir)
	PrintVerbose("Removing site: %s", filepath.Join(sitesDir, name))
	return services.DropSite(benchPath, name, config.DBRootPassword(benchPath), "--no-backup")
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
func ensureEnvironment(wegDir, frappeVersion, dbVersion string) error {
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

	// Same setup as 'weg new': socket-only mariadb/redis configs and a
	// devbox.json that pins the venv to env/. An existing devbox.json is the
	// user's and is left alone, as is a bench that already has its own env/
	// (e.g. adopted from bench): devbox would prompt to replace that venv and
	// point the bench at an empty database.
	if _, err := os.Stat(filepath.Join(wegDir, "devbox.json")); os.IsNotExist(err) {
		if fsutil.DirExists(filepath.Join(wegDir, "env")) {
			output.Warningf("%s has env/ but no devbox.json: leaving its environment to you (weg's services and site creation expect devbox)", wegDir)
			return nil
		}
		PrintInfo("Initializing development environment...")
		return initDevboxEnvironment(wegDir, frappeVersion, dbVersion)
	}

	return nil
}
