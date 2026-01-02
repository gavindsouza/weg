package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/config"
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

	result, err := config.DetectContext(cwd)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	fmt.Printf("Context: %s\n", result.ContextDescription())
	fmt.Printf("Path: %s\n", result.Path)

	if result.ConfigPath != "" {
		fmt.Printf("Config: %s\n", result.ConfigPath)
	}

	switch result.Context {
	case config.ContextWegBench:
		return showBenchConfig(result)
	case config.ContextWegApp:
		return showAppConfig(result)
	default:
		fmt.Println("\nNo weg configuration found.")
		fmt.Println(result.SuggestAction())
	}

	return nil
}

func showBenchConfig(result *config.DetectionResult) error {
	cfg, err := config.ParseWegToml(result.BenchPath)
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	fmt.Println()
	fmt.Println("[frappe]")
	fmt.Printf("  version = %q\n", cfg.Frappe.Version)
	fmt.Printf("  database = %q\n", cfg.Frappe.Database)

	fmt.Println()
	fmt.Println("[apps]")
	for name, app := range cfg.Apps {
		if app.Excluded {
			continue
		}
		if app.Path != "" {
			fmt.Printf("  %s = { path = %q }\n", name, app.Path)
		} else {
			fmt.Printf("  %s = { url = %q, branch = %q }\n", name, app.URL, app.Branch)
		}
	}

	if len(cfg.Sites) > 0 {
		fmt.Println()
		fmt.Println("[[sites]]")
		for _, site := range cfg.Sites {
			fmt.Printf("  name = %q\n", site.Name)
			if site.DefaultSite {
				fmt.Println("  default = true")
			}
		}
	}

	return nil
}

func showAppConfig(result *config.DetectionResult) error {
	pyprojectPath := filepath.Join(result.Path, "pyproject.toml")
	cfg, err := config.ParsePyproject(pyprojectPath)
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	fmt.Println()
	fmt.Println("[tool.weg.dev]")
	fmt.Printf("  frappe = %q\n", cfg.Dev.Frappe)
	fmt.Printf("  database = %q\n", cfg.Dev.Database)

	if len(cfg.Dependencies.Apps) > 0 {
		fmt.Println()
		fmt.Println("[tool.weg.dependencies]")
		for _, app := range cfg.Dependencies.Apps {
			fmt.Printf("  %s = { url = %q, branch = %q }\n", app.Name, app.URL, app.Branch)
		}
	}

	return nil
}
