package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/config"
	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Long: `Display the current project configuration.

Shows the parsed configuration from weg.toml or pyproject.toml,
including Frappe version, database, apps, and sites.`,
	RunE:         runShow,
	SilenceUsage: true,
}

func runShow(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	result, err := config.DetectProjectContext(cwd)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	output.Printf("Context: %s", result.ContextDescription())
	output.Printf("Path: %s", result.Path)

	if result.ConfigPath != "" {
		output.Printf("Config: %s", result.ConfigPath)
	}

	switch result.Context {
	case config.ContextWegBench:
		return showBenchConfig(result)
	case config.ContextWegApp:
		return showAppConfig(result)
	default:
		output.Print("\nNo weg configuration found.")
		output.Print(result.SuggestAction())
	}

	return nil
}

func showBenchConfig(result *config.DetectionResult) error {
	cfg, err := config.ParseWegToml(result.BenchPath)
	if err != nil {
		return wegerrors.Config("config", "parse", err)
	}

	output.Print("")
	output.Print("[frappe]")
	output.Printf("  version = %q", cfg.Frappe.Version)
	output.Printf("  database = %q", cfg.Frappe.Database)

	output.Print("")
	output.Print("[apps]")
	for name, app := range cfg.Apps {
		if app.Excluded {
			continue
		}
		if app.Path != "" {
			output.Printf("  %s = { path = %q }", name, app.Path)
		} else {
			output.Printf("  %s = { url = %q, branch = %q }", name, app.URL, app.Branch)
		}
	}

	if len(cfg.Sites) > 0 {
		output.Print("")
		output.Print("[[sites]]")
		for _, site := range cfg.Sites {
			output.Printf("  name = %q", site.Name)
			if site.DefaultSite {
				output.Print("  default = true")
			}
		}
	}

	return nil
}

func showAppConfig(result *config.DetectionResult) error {
	pyprojectPath := filepath.Join(result.Path, "pyproject.toml")
	cfg, err := config.ParsePyproject(pyprojectPath)
	if err != nil {
		return wegerrors.Config("config", "parse", err)
	}

	output.Print("")
	output.Print("[tool.weg.dev]")
	output.Printf("  frappe = %q", cfg.Dev.Frappe)
	output.Printf("  database = %q", cfg.Dev.Database)

	if len(cfg.Dependencies.Apps) > 0 {
		output.Print("")
		output.Print("[tool.weg.dependencies]")
		for _, app := range cfg.Dependencies.Apps {
			output.Printf("  %s = { url = %q, branch = %q }", app.Name, app.URL, app.Branch)
		}
	}

	return nil
}
