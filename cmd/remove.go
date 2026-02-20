package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/errors"
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
	result, err := config.DetectProjectContext(absPath)
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
		return errors.NotInProject(absPath)
	}
}

// removeFromAppConfig removes an app from pyproject.toml [tool.weg.dependencies]
func removeFromAppConfig(path, name string) error {
	pyprojectPath := filepath.Join(path, "pyproject.toml")

	// Read existing file
	data, err := os.ReadFile(pyprojectPath)
	if err != nil {
		return errors.Config("pyproject.toml", "read", err)
	}

	// Parse existing config
	var pyproject map[string]any
	if err := toml.Unmarshal(data, &pyproject); err != nil {
		return errors.Config("pyproject.toml", "parse", err)
	}

	// Navigate to tool.weg.dependencies.apps
	tool, ok := pyproject["tool"].(map[string]any)
	if !ok {
		return errors.NotFound("app", name)
	}

	weg, ok := tool["weg"].(map[string]any)
	if !ok {
		return errors.NotFound("app", name)
	}

	deps, ok := weg["dependencies"].(map[string]any)
	if !ok {
		return errors.NotFound("app", name)
	}

	apps, ok := deps["apps"].([]any)
	if !ok {
		return errors.NotFound("app", name)
	}

	// Find and remove the app
	found := false
	newApps := make([]any, 0, len(apps))
	for _, app := range apps {
		appMap, ok := app.(map[string]any)
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
		return errors.NotFound("app", name)
	}

	deps["apps"] = newApps

	// Write back
	f, err := os.Create(pyprojectPath)
	if err != nil {
		return errors.Config("pyproject.toml", "write", err)
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(pyproject); err != nil {
		return errors.Config("pyproject.toml", "write", err)
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
		return errors.Validation("app", "cannot remove frappe - it is required")
	}

	// Read existing file
	data, err := os.ReadFile(wegPath)
	if err != nil {
		return errors.Config("weg.toml", "read", err)
	}

	// Parse existing config
	var wegConfig map[string]any
	if err := toml.Unmarshal(data, &wegConfig); err != nil {
		return errors.Config("weg.toml", "parse", err)
	}

	// Get apps section
	apps, ok := wegConfig["apps"].(map[string]any)
	if !ok {
		return errors.NotFound("app", name)
	}

	// Check if app exists
	if _, exists := apps[name]; !exists {
		return errors.NotFound("app", name)
	}

	// Remove the app
	delete(apps, name)

	// Write back
	f, err := os.Create(wegPath)
	if err != nil {
		return errors.Config("weg.toml", "write", err)
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(wegConfig); err != nil {
		return errors.Config("weg.toml", "write", err)
	}

	PrintInfo("Removed %s from weg.toml", name)
	PrintInfo("Run 'weg sync' to uninstall the app")

	return nil
}
