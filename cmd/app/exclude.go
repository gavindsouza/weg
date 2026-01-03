package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/gavindsouza/weg/internal/completion"
	"github.com/gavindsouza/weg/internal/config"
	"github.com/spf13/cobra"
)

var excludeCmd = &cobra.Command{
	Use:   "exclude <app-name>",
	Short: "Exclude an app from sync",
	Long: `Mark an app as excluded so it won't be installed during sync.

This is useful when you want to temporarily disable an app without
removing it from the configuration.

Examples:
  weg app exclude erpnext`,
	Args:              cobra.ExactArgs(1),
	RunE:              runExclude,
	ValidArgsFunction: completion.CompleteAppNamesForArg(0),
}

func runExclude(cmd *cobra.Command, args []string) error {
	return setAppExcluded(args[0], true)
}

var includeCmd = &cobra.Command{
	Use:   "include <app-name>",
	Short: "Include an excluded app",
	Long: `Remove the excluded flag from an app so it will be installed during sync.

Examples:
  weg app include erpnext`,
	Args:              cobra.ExactArgs(1),
	RunE:              runInclude,
	ValidArgsFunction: completion.CompleteAppNamesForArg(0),
}

func runInclude(cmd *cobra.Command, args []string) error {
	return setAppExcluded(args[0], false)
}

func setAppExcluded(appName string, excluded bool) error {
	path := "."
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	result, err := config.DetectContext(absPath)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	if result.Context != config.ContextWegBench {
		return fmt.Errorf("exclude/include only works with weg.toml (bench mode)")
	}

	// Prevent excluding frappe
	if appName == "frappe" && excluded {
		return fmt.Errorf("cannot exclude frappe - it is required")
	}

	wegPath := filepath.Join(absPath, "weg.toml")

	// Read existing config
	data, err := os.ReadFile(wegPath)
	if err != nil {
		return fmt.Errorf("failed to read weg.toml: %w", err)
	}

	var wegConfig map[string]interface{}
	if err := toml.Unmarshal(data, &wegConfig); err != nil {
		return fmt.Errorf("failed to parse weg.toml: %w", err)
	}

	// Get apps section
	apps, ok := wegConfig["apps"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("no apps section in weg.toml")
	}

	// Get the app
	appConfig, ok := apps[appName].(map[string]interface{})
	if !ok {
		return fmt.Errorf("app %s not found in weg.toml", appName)
	}

	// Set/unset excluded flag
	if excluded {
		appConfig["excluded"] = true
	} else {
		delete(appConfig, "excluded")
	}

	// Write back
	f, err := os.Create(wegPath)
	if err != nil {
		return fmt.Errorf("failed to open weg.toml: %w", err)
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(wegConfig); err != nil {
		return fmt.Errorf("failed to write weg.toml: %w", err)
	}

	if excluded {
		fmt.Printf("Excluded %s from sync\n", appName)
		fmt.Println("Run 'weg sync' to apply changes")
	} else {
		fmt.Printf("Included %s in sync\n", appName)
		fmt.Println("Run 'weg sync' to install the app")
	}

	return nil
}
