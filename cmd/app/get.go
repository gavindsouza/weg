package app

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gavindsouza/weg/internal/apps"
	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:   "get <app-url-or-name> [branch]",
	Short: "Install an app",
	Long: `Install a Frappe app into the current project.

This is equivalent to 'weg add' followed by 'weg sync'.

Examples:
  weg app get https://github.com/frappe/erpnext
  weg app get frappe/erpnext
  weg app get frappe/erpnext version-15`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runGet,
}

func runGet(cmd *cobra.Command, args []string) error {
	path := "."
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

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
	appURL, appName := parseAppSpec(appSpec)

	var benchPath, appsDir string
	switch result.Context {
	case config.ContextWegBench:
		benchPath = absPath
		appsDir = filepath.Join(benchPath, "apps")
	case config.ContextWegApp:
		benchPath = filepath.Join(absPath, ".weg")
		appsDir = filepath.Join(benchPath, "apps")
	default:
		return fmt.Errorf("not a weg-managed project")
	}

	// Check if already installed
	st, err := state.Load(absPath)
	if err != nil {
		st = state.NewState()
	}

	if st.HasApp(appName) {
		return fmt.Errorf("app %s is already installed", appName)
	}

	fmt.Printf("Installing %s...\n", appName)

	// Install the app
	opts := apps.InstallOptions{
		BenchPath: benchPath,
		AppsDir:   appsDir,
		Verbose:   true,
	}

	if err := apps.InstallApp(appName, appURL, branch, opts); err != nil {
		return fmt.Errorf("failed to install %s: %w", appName, err)
	}

	// Update state
	st.AddApp(state.AppState{
		Name:   appName,
		URL:    appURL,
		Branch: branch,
	})

	if err := st.Save(absPath); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	// Also update config file
	if result.Context == config.ContextWegBench {
		if err := addAppToWegToml(absPath, appName, appURL, branch); err != nil {
			fmt.Printf("Warning: failed to update weg.toml: %v\n", err)
		}
	}

	fmt.Printf("Successfully installed %s\n", appName)
	return nil
}

func parseAppSpec(spec string) (url, name string) {
	// Check if it's a full URL
	if strings.HasPrefix(spec, "http://") || strings.HasPrefix(spec, "https://") || strings.HasPrefix(spec, "git@") {
		name = extractAppName(spec)
		return spec, name
	}

	// Check if it's a short GitHub reference (user/repo)
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+/[a-zA-Z0-9_-]+$`, spec); matched {
		url = fmt.Sprintf("https://github.com/%s", spec)
		parts := strings.Split(spec, "/")
		return url, parts[1]
	}

	// Assume it's just an app name
	return "", spec
}

func extractAppName(url string) string {
	url = strings.TrimSuffix(url, ".git")
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return url
}

func addAppToWegToml(path, name, url, branch string) error {
	// This is a simplified version - in practice would use proper TOML manipulation
	// For now, we just note that the config should be updated
	fmt.Printf("Note: Add the following to weg.toml:\n\n")
	fmt.Printf("[apps.%s]\n", name)
	fmt.Printf("url = \"%s\"\n", url)
	if branch != "" {
		fmt.Printf("branch = \"%s\"\n", branch)
	}
	fmt.Println()
	return nil
}
