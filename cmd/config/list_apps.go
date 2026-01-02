package config

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/spf13/cobra"
)

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

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "APP\tSOURCE\tBRANCH")

	for name, app := range cfg.Apps {
		if app.Excluded {
			continue
		}
		source := app.URL
		if app.Path != "" {
			source = app.Path + " (local)"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", name, source, app.Branch)
	}
	w.Flush()

	return nil
}

func listAppDeps(result *config.DetectionResult) error {
	pyprojectPath := result.Path + "/pyproject.toml"
	cfg, err := config.ParsePyproject(pyprojectPath)
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "APP\tSOURCE\tBRANCH")

	// Main app
	fmt.Fprintf(w, "%s\t%s\t%s\n", result.AppName, ". (local)", "-")

	// Dependencies
	for _, app := range cfg.Dependencies.Apps {
		fmt.Fprintf(w, "%s\t%s\t%s\n", app.Name, app.URL, app.Branch)
	}
	w.Flush()

	return nil
}
