package bench

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/config"
	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/spf13/cobra"
)

var currentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show current bench",
	Long: `Show the current bench context.

This displays information about the bench in the current directory
or the nearest parent directory that contains a weg-managed bench.

Examples:
  weg bench current
  weg -C /path/to/project bench current`,
	RunE: runCurrent,
}

func runCurrent(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Try to detect context
	result, err := config.DetectProjectContext(cwd)
	if err != nil {
		return fmt.Errorf("not in a weg-managed directory: %w", err)
	}

	var benchPath string
	var configPath string

	switch result.Context {
	case config.ContextWegBench:
		benchPath = result.BenchPath
		configPath = filepath.Join(cwd, "weg.toml")
	case config.ContextWegApp:
		benchPath = filepath.Join(cwd, ".weg")
		// Config could be in .weg/weg.toml or pyproject.toml
		if _, err := os.Stat(filepath.Join(benchPath, "weg.toml")); err == nil {
			configPath = filepath.Join(benchPath, "weg.toml")
		} else {
			configPath = filepath.Join(cwd, "pyproject.toml")
		}
	case config.ContextBench:
		benchPath = result.BenchPath
		output.Print("Traditional bench (not weg-managed)")
		output.Printf("Path: %s", benchPath)
		return nil
	case config.ContextApp:
		output.Print("Frappe app directory (not initialized with weg)")
		output.Printf("Path: %s", cwd)
		output.Print("\nRun 'weg init' to initialize weg in this directory.")
		return nil
	default:
		return wegerrors.NotFound("project", "")
	}

	output.Print("Current Bench")
	output.Print("=============")
	output.Printf("Path:   %s", benchPath)
	output.Printf("Config: %s", configPath)

	// Load and display config
	if result.Context == config.ContextWegBench {
		cfg, err := config.ParseWegToml(filepath.Dir(configPath))
		if err == nil {
			output.Print("")
			output.Printf("Name:     %s", cfg.Bench.Name)
			output.Printf("Frappe:   %s", cfg.Frappe.Version)
			output.Printf("Database: %s", cfg.Frappe.Database)

			if len(cfg.Apps) > 0 {
				output.Printf("Apps:     %d configured", len(cfg.Apps))
			}
			if len(cfg.Sites) > 0 {
				output.Printf("Sites:    %d configured", len(cfg.Sites))
			}
		}
	} else if result.Context == config.ContextWegApp {
		// Try weg.toml in .weg first (ParseWegToml takes the directory)
		if cfg, err := config.ParseWegToml(benchPath); err == nil {
			output.Print("")
			output.Printf("Name:     %s", cfg.Bench.Name)
			output.Printf("Frappe:   %s", cfg.Frappe.Version)
			output.Printf("Database: %s", cfg.Frappe.Database)
		} else {
			// Try pyproject.toml
			pyprojectPath := filepath.Join(cwd, "pyproject.toml")
			if appCfg, err := config.ParsePyproject(pyprojectPath); err == nil && appCfg != nil {
				output.Print("")
				output.Print("App-centric configuration")
				if len(appCfg.Compatibility.Frappe) > 0 {
					output.Printf("Compatible Frappe: %v", appCfg.Compatibility.Frappe)
				}
				if appCfg.Dev.Frappe != "" {
					output.Printf("Dev Frappe:        %s", appCfg.Dev.Frappe)
				}
			}
		}
	}

	return nil
}
