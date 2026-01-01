package bench

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/config"
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
	result, err := config.DetectContext(cwd)
	if err != nil {
		return fmt.Errorf("not in a weg-managed directory: %w", err)
	}

	var benchPath string
	var configPath string

	switch result.Context {
	case config.ContextWegBench:
		benchPath = cwd
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
		benchPath = cwd
		fmt.Println("Traditional bench (not weg-managed)")
		fmt.Printf("Path: %s\n", benchPath)
		return nil
	case config.ContextApp:
		fmt.Println("Frappe app directory (not initialized with weg)")
		fmt.Printf("Path: %s\n", cwd)
		fmt.Println("\nRun 'weg init' to initialize weg in this directory.")
		return nil
	default:
		return fmt.Errorf("not in a Frappe project directory")
	}

	fmt.Println("Current Bench")
	fmt.Println("=============")
	fmt.Printf("Path:   %s\n", benchPath)
	fmt.Printf("Config: %s\n", configPath)

	// Load and display config
	if result.Context == config.ContextWegBench {
		cfg, err := config.ParseWegToml(configPath)
		if err == nil {
			fmt.Println()
			fmt.Printf("Name:     %s\n", cfg.Bench.Name)
			fmt.Printf("Frappe:   %s\n", cfg.Frappe.Version)
			fmt.Printf("Database: %s\n", cfg.Frappe.Database)

			if len(cfg.Apps) > 0 {
				fmt.Printf("Apps:     %d configured\n", len(cfg.Apps))
			}
			if len(cfg.Sites) > 0 {
				fmt.Printf("Sites:    %d configured\n", len(cfg.Sites))
			}
		}
	} else if result.Context == config.ContextWegApp {
		// Try weg.toml in .weg first
		wegTomlPath := filepath.Join(benchPath, "weg.toml")
		if cfg, err := config.ParseWegToml(wegTomlPath); err == nil {
			fmt.Println()
			fmt.Printf("Name:     %s\n", cfg.Bench.Name)
			fmt.Printf("Frappe:   %s\n", cfg.Frappe.Version)
			fmt.Printf("Database: %s\n", cfg.Frappe.Database)
		} else {
			// Try pyproject.toml
			pyprojectPath := filepath.Join(cwd, "pyproject.toml")
			if appCfg, err := config.ParsePyproject(pyprojectPath); err == nil && appCfg != nil {
				fmt.Println()
				fmt.Println("App-centric configuration")
				if len(appCfg.Compatibility.Frappe) > 0 {
					fmt.Printf("Compatible Frappe: %v\n", appCfg.Compatibility.Frappe)
				}
				if appCfg.Dev.Frappe != "" {
					fmt.Printf("Dev Frappe:        %s\n", appCfg.Dev.Frappe)
				}
			}
		}
	}

	return nil
}
