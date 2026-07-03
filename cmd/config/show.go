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

	if output.EffectiveFormat() == output.FormatJSON {
		return runShowJSON(result)
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

type showFrappeInfo struct {
	Version  string `json:"version"`
	Database string `json:"database"`
}

type showAppInfo struct {
	URL      string `json:"url,omitempty"`
	Branch   string `json:"branch,omitempty"`
	Path     string `json:"path,omitempty"`
	Excluded bool   `json:"excluded,omitempty"`
}

type showSiteInfo struct {
	Name    string   `json:"name"`
	Default bool     `json:"default"`
	Apps    []string `json:"apps,omitempty"`
}

type showDepInfo struct {
	Name   string `json:"name"`
	URL    string `json:"url,omitempty"`
	Branch string `json:"branch,omitempty"`
}

type showReport struct {
	Context      string                 `json:"context"`
	Path         string                 `json:"path"`
	ConfigPath   string                 `json:"config_path,omitempty"`
	Frappe       *showFrappeInfo        `json:"frappe,omitempty"`
	Dev          *showFrappeInfo        `json:"dev,omitempty"`
	Apps         map[string]showAppInfo `json:"apps,omitempty"`
	Dependencies []showDepInfo          `json:"dependencies,omitempty"`
	Sites        []showSiteInfo         `json:"sites,omitempty"`
}

// runShowJSON emits the current configuration as machine-readable JSON.
func runShowJSON(result *config.DetectionResult) error {
	report := showReport{
		Context:    result.ContextDescription(),
		Path:       result.Path,
		ConfigPath: result.ConfigPath,
	}

	switch result.Context {
	case config.ContextWegBench:
		cfg, err := config.ParseWegToml(result.BenchPath)
		if err != nil {
			return wegerrors.Config("config", "parse", err)
		}
		report.Frappe = &showFrappeInfo{Version: cfg.Frappe.Version, Database: cfg.Frappe.Database}
		report.Apps = make(map[string]showAppInfo, len(cfg.Apps))
		for name, app := range cfg.Apps {
			report.Apps[name] = showAppInfo{URL: app.URL, Branch: app.Branch, Path: app.Path, Excluded: app.Excluded}
		}
		for _, site := range cfg.Sites {
			report.Sites = append(report.Sites, showSiteInfo{Name: site.Name, Default: site.DefaultSite, Apps: site.Apps})
		}

	case config.ContextWegApp:
		pyprojectPath := filepath.Join(result.Path, "pyproject.toml")
		cfg, err := config.ParsePyproject(pyprojectPath)
		if err != nil {
			return wegerrors.Config("config", "parse", err)
		}
		report.Dev = &showFrappeInfo{Version: cfg.Dev.Frappe, Database: cfg.Dev.Database}
		for _, app := range cfg.Dependencies.Apps {
			report.Dependencies = append(report.Dependencies, showDepInfo{Name: app.Name, URL: app.URL, Branch: app.Branch})
		}
	}

	return output.JSON(report)
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
