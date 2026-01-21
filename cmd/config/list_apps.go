package config

import (
	"fmt"
	"os"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/spf13/cobra"
)

// AppListItem represents an app for list output
type AppListItem struct {
	App    string `json:"app"`
	Source string `json:"source"`
	Branch string `json:"branch"`
}

var listAppsCmd = &cobra.Command{
	Use:   "list-apps",
	Short: "List configured apps",
	Long: `List all apps configured in the current project.

Shows app name, source (URL or path), and branch/version.`,
	RunE:         runListApps,
	SilenceUsage: true,
}

func runListApps(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	result, err := config.DetectContext(cwd)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	switch result.Context {
	case config.ContextWegBench:
		return listBenchApps(result)
	case config.ContextWegApp:
		return listAppDeps(result)
	default:
		return fmt.Errorf("not a weg-managed project")
	}
}

func listBenchApps(result *config.DetectionResult) error {
	cfg, err := config.ParseWegToml(result.BenchPath)
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	var apps []AppListItem
	for name, app := range cfg.Apps {
		if app.Excluded {
			continue
		}
		source := app.URL
		if app.Path != "" {
			source = app.Path + " (local)"
		}
		apps = append(apps, AppListItem{
			App:    name,
			Source: source,
			Branch: app.Branch,
		})
	}

	return output.List(apps)
}

func listAppDeps(result *config.DetectionResult) error {
	pyprojectPath := result.Path + "/pyproject.toml"
	cfg, err := config.ParsePyproject(pyprojectPath)
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	var apps []AppListItem

	// Main app
	apps = append(apps, AppListItem{
		App:    result.AppName,
		Source: ". (local)",
		Branch: "-",
	})

	// Dependencies
	for _, app := range cfg.Dependencies.Apps {
		apps = append(apps, AppListItem{
			App:    app.Name,
			Source: app.URL,
			Branch: app.Branch,
		})
	}

	return output.List(apps)
}
