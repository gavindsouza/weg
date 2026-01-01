package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/gavindsouza/weg/internal/config"
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
	result, err := config.DetectContext(absPath)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	appSpec := args[0]
	branch := ""
	if len(args) > 1 {
		branch = args[1]
	}

	// Parse app specification
	appURL, appName, isLocal := parseAppSpec(appSpec)

	switch result.Context {
	case config.ContextWegApp:
		return addToAppConfig(absPath, appName, appURL, branch, isLocal)
	case config.ContextWegBench:
		return addToBenchConfig(absPath, appName, appURL, branch, isLocal)
	default:
		return fmt.Errorf("not a weg-managed project. Run 'weg init' first")
	}
}

// parseAppSpec parses an app specification and returns URL, name, and whether it's local
func parseAppSpec(spec string) (url, name string, isLocal bool) {
	// Check if it's a local path
	if strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "/") || strings.HasPrefix(spec, "..") {
		absPath, _ := filepath.Abs(spec)
		return absPath, filepath.Base(absPath), true
	}

	// Check if it's a full URL
	if strings.HasPrefix(spec, "http://") || strings.HasPrefix(spec, "https://") || strings.HasPrefix(spec, "git@") {
		name = extractAppName(spec)
		return spec, name, false
	}

	// Check if it's a short GitHub reference (user/repo)
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+/[a-zA-Z0-9_-]+$`, spec); matched {
		url = fmt.Sprintf("https://github.com/%s", spec)
		parts := strings.Split(spec, "/")
		return url, parts[1], false
	}

	// Assume it's just an app name (for future marketplace support)
	return "", spec, false
}

// extractAppName extracts the app name from a git URL
func extractAppName(url string) string {
	// Remove .git suffix
	url = strings.TrimSuffix(url, ".git")

	// Get last part of URL
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}

	return url
}

// addToAppConfig adds an app to pyproject.toml [tool.weg.dependencies]
func addToAppConfig(path, name, url, branch string, isLocal bool) error {
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

	// Navigate to or create tool.weg.dependencies.apps
	tool, ok := pyproject["tool"].(map[string]interface{})
	if !ok {
		tool = make(map[string]interface{})
		pyproject["tool"] = tool
	}

	weg, ok := tool["weg"].(map[string]interface{})
	if !ok {
		weg = make(map[string]interface{})
		tool["weg"] = weg
	}

	deps, ok := weg["dependencies"].(map[string]interface{})
	if !ok {
		deps = make(map[string]interface{})
		weg["dependencies"] = deps
	}

	apps, ok := deps["apps"].([]interface{})
	if !ok {
		apps = []interface{}{}
	}

	// Add new app
	newApp := map[string]interface{}{
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
		return fmt.Errorf("failed to open pyproject.toml for writing: %w", err)
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(pyproject); err != nil {
		return fmt.Errorf("failed to write pyproject.toml: %w", err)
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
		return fmt.Errorf("failed to read weg.toml: %w", err)
	}

	// Parse existing config
	var wegConfig map[string]interface{}
	if err := toml.Unmarshal(data, &wegConfig); err != nil {
		return fmt.Errorf("failed to parse weg.toml: %w", err)
	}

	// Get or create apps section
	apps, ok := wegConfig["apps"].(map[string]interface{})
	if !ok {
		apps = make(map[string]interface{})
		wegConfig["apps"] = apps
	}

	// Add new app
	appConfig := make(map[string]interface{})
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
		return fmt.Errorf("failed to open weg.toml for writing: %w", err)
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(wegConfig); err != nil {
		return fmt.Errorf("failed to write weg.toml: %w", err)
	}

	PrintInfo("Added %s to weg.toml", name)
	PrintInfo("Run 'weg sync' to install the app")

	return nil
}
