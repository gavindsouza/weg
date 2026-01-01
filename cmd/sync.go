package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/apps"
	"github.com/gavindsouza/weg/internal/config"
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
	RunE: runSync,
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

	// Parse config
	appConfig, err := config.ParsePyproject(path)
	if err != nil {
		return fmt.Errorf("failed to parse pyproject.toml: %w", err)
	}

	// Validate config
	if err := config.ValidateAppConfig(appConfig); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Load state
	st, err := state.Load(path)
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
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
	configHash, _ := state.ComputeConfigHash(filepath.Join(path, "pyproject.toml"))
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
	benchConfig, err := config.ParseWegToml(path)
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
	diff := state.ComputeDiffFromBenchConfig(benchConfig, st)

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
}

func applyAppChanges(path string, cfg *config.AppConfig, st *state.State, diff *state.Diff) error {
	// For app-centric, we need to create/update the .weg directory
	wegDir := filepath.Join(path, ".weg")

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

		st.AddApp(state.AppState{Name: appName})
	}

	// Remove apps
	for _, appName := range diff.AppsToRemove {
		PrintInfo("Removing %s...", appName)
		if err := removeApp(wegDir, appName); err != nil {
			PrintVerbose("Warning: failed to remove %s: %v", appName, err)
		}
		st.RemoveApp(appName)
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

		st.AddApp(state.AppState{
			Name:   appName,
			URL:    appCfg.URL,
			Branch: appCfg.Branch,
			Path:   appCfg.Path,
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

		st.Apps[update.Name] = state.AppState{
			Name:   update.Name,
			URL:    update.NewURL,
			Branch: update.NewBranch,
		}
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
			if err := createSite(sitesDir, siteCfg); err != nil {
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

	return nil
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

func createSite(sitesDir string, cfg *config.SiteConfig) error {
	// Create site directory
	sitePath := filepath.Join(sitesDir, cfg.Name)
	if err := os.MkdirAll(sitePath, 0755); err != nil {
		return fmt.Errorf("failed to create site directory: %w", err)
	}

	// TODO: Run actual site creation via frappe
	// For now, just create the directory structure
	PrintVerbose("Created site directory: %s", sitePath)
	PrintVerbose("Note: Run 'bench new-site %s' to complete site setup", cfg.Name)

	return nil
}

func removeSite(sitesDir, name string) error {
	sitePath := filepath.Join(sitesDir, name)

	// TODO: Drop database before removing directory
	PrintVerbose("Removing site: %s", sitePath)

	return os.RemoveAll(sitePath)
}
