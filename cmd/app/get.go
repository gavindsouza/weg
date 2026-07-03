package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/gavindsouza/weg/internal/apps"
	"github.com/gavindsouza/weg/internal/config"
	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var getSkipDeps bool

var getCmd = &cobra.Command{
	Use:   "get <app-url-or-name> [branch]",
	Short: "Install an app",
	Long: `Install a Frappe app into the current project.

Automatically resolves and installs transitive dependencies.
Use --skip-deps to install only the specified app.

Unlike 'weg add' (which only edits the configuration for a later
'weg sync'), this clones and installs the app immediately.

Examples:
  weg app get https://github.com/frappe/erpnext
  weg app get frappe/erpnext
  weg app get frappe/erpnext version-15
  weg app get frappe/hrms --skip-deps`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runGet,
}

func init() {
	getCmd.Flags().BoolVar(&getSkipDeps, "skip-deps", false, "Skip dependency resolution; install only the specified app")
}

func runGet(cmd *cobra.Command, args []string) error {
	path := "."
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	result, err := config.DetectProjectContext(absPath)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	rawSpec := args[0]
	branch := ""
	if len(args) > 1 {
		branch = args[1]
	}

	// Parse app specification using shared resolver
	appSpec := apps.ResolveAppSpec(rawSpec, branch)
	appName := appSpec.Name
	appURL := appSpec.URL

	if !result.IsWegManaged() {
		return wegerrors.NotInProject(absPath)
	}
	benchPath := result.BenchPath
	appsDir := filepath.Join(benchPath, "apps")

	// Check if already installed
	st, err := state.Load(absPath)
	if err != nil {
		st = state.NewState()
	}

	if st.HasApp(appName) {
		return wegerrors.Validation("app", fmt.Sprintf("%s is already installed", appName))
	}

	output.Infof("Installing %s...", appName)

	// Install the app
	opts := apps.InstallOptions{
		BenchPath: benchPath,
		AppsDir:   appsDir,
		Verbose:   true,
	}

	if err := apps.InstallApp(appName, appURL, appSpec.Branch, opts); err != nil {
		return fmt.Errorf("failed to install %s: %w", appName, err)
	}

	// Update state
	st.AddApp(state.AppState{
		Name:   appName,
		URL:    appURL,
		Branch: appSpec.Branch,
	})

	if err := st.Save(absPath); err != nil {
		return wegerrors.State("save", err)
	}

	// Resolve and install transitive dependencies (default behavior)
	if !getSkipDeps {
		output.Infof("Resolving dependencies for %s...", appName)

		installed := make(map[string]bool)
		installed[appName] = true
		installed["frappe"] = true
		for name := range st.Apps {
			installed[name] = true
		}

		resolveOpts := apps.ResolveOptions{
			BenchPath:     benchPath,
			AppsDir:       appsDir,
			AllowRemote:   true,
			InstalledApps: installed,
			Verbose:       true,
			LogFunc:       output.Infof,
		}

		resolveResult, err := apps.ResolveDependencies(appSpec, resolveOpts)
		if err != nil {
			output.Warningf("Dependency resolution failed: %v", err)
			output.Warningf("The app itself was installed successfully; install dependencies manually")
		} else {
			apps.PrintResolveResult(resolveResult)

			for _, dep := range resolveResult.InstallOrder {
				output.Infof("Installing dependency: %s...", dep.Name)
				if err := apps.InstallApp(dep.Name, dep.URL, dep.Branch, opts); err != nil {
					output.Warningf("Failed to install dependency %s: %v", dep.Name, err)
					continue
				}
				st.AddApp(state.AppState{
					Name:   dep.Name,
					URL:    dep.URL,
					Branch: dep.Branch,
				})
				if err := st.Save(absPath); err != nil {
					output.Warningf("Failed to save state after installing %s: %v", dep.Name, err)
				}
				if result.Context == config.ContextWegBench {
					if err := addAppToWegToml(absPath, dep.Name, dep.URL, dep.Branch); err != nil {
						output.Warningf("Failed to update weg.toml for %s: %v", dep.Name, err)
					}
				}
				output.Successf("Installed dependency %s", dep.Name)
			}
		}
	}

	// Also update config file
	if result.Context == config.ContextWegBench {
		if err := addAppToWegToml(absPath, appName, appURL, appSpec.Branch); err != nil {
			output.Warningf("Failed to update weg.toml: %v", err)
		}
	}

	output.Successf("Installed %s", appName)
	return nil
}

func addAppToWegToml(path, name, url, branch string) error {
	wegPath := filepath.Join(path, "weg.toml")
	cfg, err := config.ParseWegToml(path)
	if err != nil {
		return wegerrors.Config("weg.toml", "read", err)
	}

	if cfg.Apps == nil {
		cfg.Apps = make(map[string]config.AppSettings)
	}
	cfg.Apps[name] = config.AppSettings{
		URL:    url,
		Branch: branch,
	}

	f, err := os.Create(wegPath)
	if err != nil {
		return fmt.Errorf("failed to write weg.toml: %w", err)
	}
	defer f.Close()

	if err := toml.NewEncoder(f).Encode(cfg); err != nil {
		return wegerrors.Config("weg.toml", "write", err)
	}

	return nil
}
