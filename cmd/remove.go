package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/gavindsouza/weg/internal/config"
	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:     "remove <app-name>",
	Aliases: []string{"rm"},
	Short:   "Remove an app from the project",
	Long: `Remove a Frappe app from the current project configuration.

This removes the app from the configuration file (weg.toml or pyproject.toml).
Run 'weg sync' afterwards to actually uninstall the app.

Examples:
  weg remove erpnext
  weg rm erpnext`,
	Args: cobra.ExactArgs(1),
	RunE: runRemove,
}

func init() {
	rootCmd.AddCommand(removeCmd)
}

func runRemove(cmd *cobra.Command, args []string) error {
	path := "."
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Detect context
	result, err := config.DetectContext(absPath)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	appName := args[0]

	switch result.Context {
	case config.ContextWegApp:
		return removeFromAppConfig(absPath, appName)
	case config.ContextWegBench:
		return removeFromBenchConfig(absPath, appName)
	default:
		return fmt.Errorf("not a weg-managed project. Run 'weg init' first")
	}
}

// removeFromAppConfig removes an app from pyproject.toml [tool.weg.dependencies]
func removeFromAppConfig(path, name string) error {
	pyprojectPath := filepath.Join(path, "pyproject.toml")

	// Read existing file
	data, err := os.ReadFile(pyprojectPath)
	if err != nil {
		return fmt.Errorf("failed to read pyproject.toml: %w", err)
	}

	// Parse existing config
	var pyproject map[string]interface{}
	if err := toml.Unmarshal(data, &pyproject); err != nil {
		return fmt.Errorf("failed to parse pyproject.toml: %w", err)
	}

	// Navigate to tool.weg.dependencies.apps
	tool, ok := pyproject["tool"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("app %s not found in configuration", name)
	}

	weg, ok := tool["weg"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("app %s not found in configuration", name)
	}

	deps, ok := weg["dependencies"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("app %s not found in configuration", name)
	}

	apps, ok := deps["apps"].([]interface{})
	if !ok {
		return fmt.Errorf("app %s not found in configuration", name)
	}

	// Find and remove the app
	found := false
	newApps := make([]interface{}, 0, len(apps))
	for _, app := range apps {
		appMap, ok := app.(map[string]interface{})
		if !ok {
			newApps = append(newApps, app)
			continue
		}

		appName, ok := appMap["name"].(string)
		if ok && appName == name {
			found = true
			continue
		}

		newApps = append(newApps, app)
	}

	if !found {
		return fmt.Errorf("app %s not found in configuration", name)
	}

	deps["apps"] = newApps

	// Write back
	f, err := os.Create(pyprojectPath)
	if err != nil {
		return fmt.Errorf("failed to open pyproject.toml for writing: %w", err)
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(pyproject); err != nil {
		return fmt.Errorf("failed to write pyproject.toml: %w", err)
	}

	PrintInfo("Removed %s from pyproject.toml", name)
	PrintInfo("Run 'weg sync' to uninstall the app")

	return nil
}

// removeFromBenchConfig removes an app from weg.toml [apps]
func removeFromBenchConfig(path, name string) error {
	wegPath := filepath.Join(path, "weg.toml")

	// Prevent removing frappe
	if name == "frappe" {
		return fmt.Errorf("cannot remove frappe - it is required")
	}

	// Read existing file
	data, err := os.ReadFile(wegPath)
	if err != nil {
		return fmt.Errorf("failed to read weg.toml: %w", err)
	}

	// Parse existing config
	var wegConfig map[string]interface{}
	if err := toml.Unmarshal(data, &wegConfig); err != nil {
		return fmt.Errorf("failed to parse weg.toml: %w", err)
	}

	// Get apps section
	apps, ok := wegConfig["apps"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("app %s not found in configuration", name)
	}

	// Check if app exists
	if _, exists := apps[name]; !exists {
		return fmt.Errorf("app %s not found in configuration", name)
	}

	// Remove the app
	delete(apps, name)

	// Write back
	f, err := os.Create(wegPath)
	if err != nil {
		return fmt.Errorf("failed to open weg.toml for writing: %w", err)
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(wegConfig); err != nil {
		return fmt.Errorf("failed to write weg.toml: %w", err)
	}

	PrintInfo("Removed %s from weg.toml", name)
	PrintInfo("Run 'weg sync' to uninstall the app")

	return nil
}
