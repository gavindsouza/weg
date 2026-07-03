package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/gavindsouza/weg/internal/apps"
	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/errors"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add <app-url-or-name> [branch]",
	Short: "Add an app to the project",
	Long: `Add a Frappe app to the current project.

The app can be specified as:
  - A GitHub URL: https://github.com/frappe/erpnext
  - A short name: frappe/erpnext (expands to GitHub URL)
  - An app name from Frappe Cloud marketplace (future)

'weg add' only edits the configuration (weg.toml or pyproject.toml);
run 'weg sync' afterwards to install. To clone and install an app
immediately, see 'weg app get'.

Examples:
  weg add https://github.com/frappe/erpnext
  weg add frappe/erpnext
  weg add frappe/erpnext version-15
  weg add ./path/to/local/app`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runAdd,
}

func init() {
	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
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

	rawSpec := args[0]
	branch := ""
	if len(args) > 1 {
		branch = args[1]
	}

	// Parse app specification using shared resolver
	appSpec := apps.ResolveAppSpec(rawSpec, branch)

	switch result.Context {
	case config.ContextWegApp:
		return addToAppConfig(absPath, appSpec.Name, appSpec.URL, appSpec.Branch, appSpec.IsLocal)
	case config.ContextWegBench:
		return addToBenchConfig(absPath, appSpec.Name, appSpec.URL, appSpec.Branch, appSpec.IsLocal)
	default:
		return errors.NotInProject(absPath)
	}
}

// addToAppConfig adds an app to pyproject.toml [tool.weg.dependencies]
func addToAppConfig(path, name, url, branch string, isLocal bool) error {
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

	// Navigate to or create tool.weg.dependencies.apps
	tool, ok := pyproject["tool"].(map[string]any)
	if !ok {
		tool = make(map[string]any)
		pyproject["tool"] = tool
	}

	weg, ok := tool["weg"].(map[string]any)
	if !ok {
		weg = make(map[string]any)
		tool["weg"] = weg
	}

	deps, ok := weg["dependencies"].(map[string]any)
	if !ok {
		deps = make(map[string]any)
		weg["dependencies"] = deps
	}

	apps, ok := deps["apps"].([]any)
	if !ok {
		apps = []any{}
	}

	// Add new app
	newApp := map[string]any{
		"name": name,
	}
	if url != "" {
		newApp["url"] = url
	}
	if branch != "" {
		newApp["branch"] = branch
	}

	apps = append(apps, newApp)
	deps["apps"] = apps

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

	PrintInfo("Added %s to pyproject.toml", name)
	PrintInfo("Run 'weg sync' to install the app")

	return nil
}

// addToBenchConfig adds an app to weg.toml [apps]
func addToBenchConfig(path, name, url, branch string, isLocal bool) error {
	wegPath := filepath.Join(path, "weg.toml")

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

	// Get or create apps section
	apps, ok := wegConfig["apps"].(map[string]any)
	if !ok {
		apps = make(map[string]any)
		wegConfig["apps"] = apps
	}

	// Add new app
	appConfig := make(map[string]any)
	if isLocal {
		appConfig["path"] = url
	} else {
		if url != "" {
			appConfig["url"] = url
		}
		if branch != "" {
			appConfig["branch"] = branch
		}
	}

	apps[name] = appConfig

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

	PrintInfo("Added %s to weg.toml", name)
	PrintInfo("Run 'weg sync' to install the app")

	return nil
}
