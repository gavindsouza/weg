package app

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/gavindsouza/weg/internal/apps"
	"github.com/gavindsouza/weg/internal/config"
	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/output"
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

	result, err := config.DetectProjectContext(absPath)
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

	if !result.IsWegManaged() {
		return wegerrors.NotInProject(absPath)
	}
	benchPath := result.BenchPath
	appsDir := filepath.Join(benchPath, "apps")

	// Check if already installed
	st, err := state.Load(absPath)
	if err != nil {
		st = state.NewState()
	}

	if st.HasApp(appName) {
		return fmt.Errorf("app %s is already installed", appName)
	}

	output.Infof("Installing %s...", appName)

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
			output.Warningf("Failed to update weg.toml: %v", err)
		}
	}

	output.Successf("Installed %s", appName)
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
	wegPath := filepath.Join(path, "weg.toml")
	cfg, err := config.ParseWegToml(path)
	if err != nil {
		return fmt.Errorf("failed to read weg.toml: %w", err)
	}

	if cfg.Apps == nil {
		cfg.Apps = make(map[string]config.AppSettings)
	}
	cfg.Apps[name] = config.AppSettings{
		URL:    url,
		Branch: branch,
	}

	f, err := os.Create(wegPath)
	if err != nil {
		return fmt.Errorf("failed to write weg.toml: %w", err)
	}
	defer f.Close()

	if err := toml.NewEncoder(f).Encode(cfg); err != nil {
		return fmt.Errorf("failed to encode weg.toml: %w", err)
	}

	return nil
}
