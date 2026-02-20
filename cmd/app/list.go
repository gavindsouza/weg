package app

import (
	"fmt"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/apps"
	"github.com/gavindsouza/weg/internal/config"
	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List apps in the project",
	Long: `List all apps configured in the project.

Shows both configured apps (from weg.toml or pyproject.toml) and
their installation status.

Examples:
  weg app list
  weg app ls`,
	RunE: runList,
}

func runList(cmd *cobra.Command, args []string) error {
	path := "."
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	result, err := config.DetectProjectContext(absPath)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	var benchPath, appsDir string
	var configuredApps map[string]config.AppSettings

	switch result.Context {
	case config.ContextWegBench:
		benchPath = result.BenchPath
		appsDir = filepath.Join(benchPath, "apps")
		benchConfig, err := config.ParseWegToml(absPath)
		if err != nil {
			return wegerrors.Config("weg.toml", "parse", err)
		}
		configuredApps = benchConfig.Apps

	case config.ContextWegApp:
		benchPath = result.BenchPath
		appsDir = filepath.Join(benchPath, "apps")
		// For app-centric, show the main app and dependencies
		appConfig, err := config.ParsePyproject(absPath)
		if err != nil {
			return wegerrors.Config("pyproject.toml", "parse", err)
		}
		configuredApps = make(map[string]config.AppSettings)
		configuredApps["frappe"] = config.AppSettings{
			URL:    "https://github.com/frappe/frappe",
			Branch: "version-" + appConfig.Dev.Frappe,
		}
		configuredApps[filepath.Base(absPath)] = config.AppSettings{
			Path: absPath,
		}
		for _, dep := range appConfig.Dependencies.Apps {
			configuredApps[dep.Name] = config.AppSettings{
				URL:    dep.URL,
				Branch: dep.Branch,
			}
		}

	default:
		return wegerrors.NotInProject(absPath)
	}

	// Load state
	st, err := state.Load(absPath)
	if err != nil {
		st = state.NewState()
	}

	// Build list of apps for output
	type AppInfo struct {
		Name   string `json:"name"`
		Branch string `json:"branch"`
		Status string `json:"status"`
		Source string `json:"source"`
	}

	var appList []AppInfo
	for name, appCfg := range configuredApps {
		branch := appCfg.Branch
		if branch == "" && appCfg.Path != "" {
			branch = "(local)"
		}

		status := "not installed"
		if st.HasApp(name) {
			appPath := filepath.Join(appsDir, name)
			if apps.IsGitRepo(appPath) {
				status = "installed"
				// Get actual branch
				if actualBranch, err := apps.GetCurrentBranch(appPath); err == nil {
					if branch != "" && actualBranch != branch {
						status = fmt.Sprintf("installed (%s)", actualBranch)
					}
				}
			}
		}

		source := appCfg.URL
		if appCfg.Path != "" {
			source = appCfg.Path
		}
		if len(source) > 50 {
			source = "..." + source[len(source)-47:]
		}

		if appCfg.Excluded {
			status = "excluded"
		}

		appList = append(appList, AppInfo{
			Name:   name,
			Branch: branch,
			Status: status,
			Source: source,
		})
	}

	return output.List(appList)
}
