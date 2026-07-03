package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/errors"
	"github.com/spf13/cobra"
)

var convertCmd = &cobra.Command{
	Use:   "convert <app|bench>",
	Short: "Convert between app-centric and bench-centric modes",
	Long: `Convert your project structure between development modes.

Modes:
  app    - App-centric: Your app is the root, bench is hidden in .weg/
  bench  - Bench-centric: Traditional bench structure with apps/ and sites/

For database migrations, see 'weg db migrate'.

Examples:
  weg convert bench    # Convert app-centric to bench-centric
  weg convert app      # Convert bench-centric to app-centric`,
	Args:         cobra.ExactArgs(1),
	RunE:         runConvert,
	SilenceUsage: true,
}

func init() {
	rootCmd.AddCommand(convertCmd)
}

func runConvert(cmd *cobra.Command, args []string) error {
	targetMode := strings.ToLower(args[0])
	if targetMode != "app" && targetMode != "bench" {
		return errors.Validation("mode", fmt.Sprintf("must be 'app' or 'bench', got %s", targetMode))
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	result, err := config.DetectProjectContext(cwd)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	switch targetMode {
	case "bench":
		return convertToBench(cwd, result)
	case "app":
		return convertToApp(cwd, result)
	}

	return nil
}

func convertToBench(cwd string, result *config.DetectionResult) error {
	if result.Context == config.ContextWegBench {
		PrintInfo("Already in bench-centric mode.")
		return nil
	}

	if result.Context != config.ContextWegApp {
		return errors.Usagef("can only convert from app-centric mode. Current: %s", result.Context.String())
	}

	wegDir := filepath.Join(cwd, ".weg")
	appName := result.AppName

	PrintInfo("Converting to bench-centric mode...")

	// Read current weg.toml
	cfg, err := config.ParseWegToml(wegDir)
	if err != nil {
		return errors.Config("config", "read", err)
	}

	// Create new bench directory structure at current level
	// Move .weg contents up, update paths

	// 1. Move apps/ to current directory
	srcApps := filepath.Join(wegDir, "apps")
	dstApps := filepath.Join(cwd, "apps")
	if err := os.Rename(srcApps, dstApps); err != nil {
		return fmt.Errorf("failed to move apps: %w", err)
	}

	// 2. Move sites/ to current directory
	srcSites := filepath.Join(wegDir, "sites")
	dstSites := filepath.Join(cwd, "sites")
	if err := os.Rename(srcSites, dstSites); err != nil {
		return fmt.Errorf("failed to move sites: %w", err)
	}

	// 3. Move config/ if exists
	srcConfig := filepath.Join(wegDir, "config")
	dstConfig := filepath.Join(cwd, "config")
	if _, err := os.Stat(srcConfig); err == nil {
		if err := os.Rename(srcConfig, dstConfig); err != nil {
			return fmt.Errorf("failed to move config: %w", err)
		}
	}

	// 4. Move logs/ if exists
	srcLogs := filepath.Join(wegDir, "logs")
	dstLogs := filepath.Join(cwd, "logs")
	if _, err := os.Stat(srcLogs); err == nil {
		if err := os.Rename(srcLogs, dstLogs); err != nil {
			return fmt.Errorf("failed to move logs: %w", err)
		}
	}

	// 5. Update weg.toml - change relative paths to absolute or adjust
	// The main app symlink pointed to ".." which was the app root
	// Now it should point to the actual app location
	if appCfg, ok := cfg.Apps[toModuleName(appName)]; ok {
		if appCfg.Path == ".." {
			// The app is the current directory, update to point to it
			appCfg.Path = cwd
			cfg.Apps[toModuleName(appName)] = appCfg
		}
	}

	// 6. Write weg.toml to current directory
	if err := writeWegToml(filepath.Join(cwd, "weg.toml"), cfg); err != nil {
		return errors.Config("weg.toml", "write", err)
	}

	// 7. Move devbox files
	srcDevbox := filepath.Join(wegDir, "devbox.json")
	dstDevbox := filepath.Join(cwd, "devbox.json")
	if _, err := os.Stat(srcDevbox); err == nil {
		if err := os.Rename(srcDevbox, dstDevbox); err != nil {
			return fmt.Errorf("failed to move devbox.json: %w", err)
		}
	}

	srcEnvrc := filepath.Join(wegDir, ".envrc")
	dstEnvrc := filepath.Join(cwd, ".envrc")
	if _, err := os.Stat(srcEnvrc); err == nil {
		if err := os.Rename(srcEnvrc, dstEnvrc); err != nil {
			return fmt.Errorf("failed to move .envrc: %w", err)
		}
	}

	// 8. Move env/ (venv)
	srcEnv := filepath.Join(wegDir, "env")
	dstEnv := filepath.Join(cwd, "env")
	if _, err := os.Stat(srcEnv); err == nil {
		if err := os.Rename(srcEnv, dstEnv); err != nil {
			return fmt.Errorf("failed to move env: %w", err)
		}
	}

	// 9. Clean up .weg directory
	if err := os.RemoveAll(wegDir); err != nil {
		PrintVerbose("Warning: failed to remove .weg: %v", err)
	}

	// 10. Fix the app symlink - remove old one, create proper one
	appSymlink := filepath.Join(dstApps, toModuleName(appName))
	if info, err := os.Lstat(appSymlink); err == nil && info.Mode()&os.ModeSymlink != 0 {
		os.Remove(appSymlink)
		// Create symlink to current directory
		if err := os.Symlink(cwd, appSymlink); err != nil {
			return fmt.Errorf("failed to update app symlink: %w", err)
		}
	}

	PrintInfo("Converted to bench-centric mode")
	PrintInfo("")
	PrintInfo("Your bench is now at: %s", cwd)
	PrintInfo("Run 'direnv allow' to activate the environment")

	return nil
}

func convertToApp(cwd string, result *config.DetectionResult) error {
	if result.Context == config.ContextWegApp {
		PrintInfo("Already in app-centric mode.")
		return nil
	}

	if result.Context != config.ContextWegBench {
		return errors.Usagef("can only convert from bench-centric mode. Current: %s", result.Context.String())
	}

	PrintInfo("Converting to app-centric mode...")
	PrintInfo("")
	PrintInfo("This will move the bench into .weg/ inside your primary app.")

	// Find the primary app (non-frappe, local or first custom app)
	cfg, err := config.ParseWegToml(cwd)
	if err != nil {
		return errors.Config("config", "read", err)
	}

	var primaryApp string
	var primaryAppPath string
	for name, appCfg := range cfg.Apps {
		if name == "frappe" {
			continue
		}
		if appCfg.Path != "" {
			// Local app - this is likely the primary
			primaryApp = name
			primaryAppPath = appCfg.Path
			break
		}
		if primaryApp == "" {
			primaryApp = name
		}
	}

	if primaryApp == "" {
		return fmt.Errorf("no custom apps found to migrate to")
	}

	// If no local path, the app is in apps/
	if primaryAppPath == "" {
		primaryAppPath = filepath.Join(cwd, "apps", primaryApp)
	}

	// Resolve to absolute path
	if !filepath.IsAbs(primaryAppPath) {
		primaryAppPath = filepath.Join(cwd, primaryAppPath)
	}

	PrintInfo("Primary app: %s at %s", primaryApp, primaryAppPath)

	// Create .weg in the app directory
	wegDir := filepath.Join(primaryAppPath, ".weg")
	if err := os.MkdirAll(wegDir, 0755); err != nil {
		return fmt.Errorf("failed to create .weg: %w", err)
	}

	// Move bench contents to .weg
	moves := []struct{ src, dst string }{
		{"apps", "apps"},
		{"sites", "sites"},
		{"config", "config"},
		{"logs", "logs"},
		{"env", "env"},
		{"devbox.json", "devbox.json"},
		{".envrc", ".envrc"},
		{"devbox.d", "devbox.d"},
		{".devbox", ".devbox"},
	}

	for _, m := range moves {
		src := filepath.Join(cwd, m.src)
		dst := filepath.Join(wegDir, m.dst)
		if _, err := os.Stat(src); err == nil {
			if err := os.Rename(src, dst); err != nil {
				PrintVerbose("Warning: failed to move %s: %v", m.src, err)
			}
		}
	}

	// Update weg.toml - change app path to ".."
	if appCfg, ok := cfg.Apps[primaryApp]; ok {
		appCfg.Path = ".."
		cfg.Apps[primaryApp] = appCfg
	}

	// Write weg.toml to .weg
	if err := writeWegToml(filepath.Join(wegDir, "weg.toml"), cfg); err != nil {
		return errors.Config("weg.toml", "write", err)
	}

	// Remove old weg.toml from bench root
	os.Remove(filepath.Join(cwd, "weg.toml"))

	// Fix app symlink
	appSymlink := filepath.Join(wegDir, "apps", primaryApp)
	if info, err := os.Lstat(appSymlink); err == nil && info.Mode()&os.ModeSymlink != 0 {
		os.Remove(appSymlink)
	}
	if err := os.Symlink(primaryAppPath, appSymlink); err != nil {
		return fmt.Errorf("failed to create app symlink: %w", err)
	}

	PrintInfo("Converted to app-centric mode")
	PrintInfo("")
	PrintInfo("Your app is now the project root: %s", primaryAppPath)
	PrintInfo("The bench is hidden in: %s", wegDir)
	PrintInfo("")
	PrintInfo("Next: cd %s && direnv allow", primaryAppPath)

	return nil
}

func writeWegToml(path string, cfg *config.BenchConfig) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Write bench section
	fmt.Fprintf(f, "[bench]\n")
	fmt.Fprintf(f, "name = %q\n", cfg.Bench.Name)
	fmt.Fprintf(f, "\n")

	// Write frappe section
	fmt.Fprintf(f, "[frappe]\n")
	fmt.Fprintf(f, "version = %q\n", cfg.Frappe.Version)
	fmt.Fprintf(f, "database = %q\n", cfg.Frappe.Database)
	fmt.Fprintf(f, "\n")

	// Write apps
	for name, app := range cfg.Apps {
		fmt.Fprintf(f, "[apps.%s]\n", name)
		if app.URL != "" {
			fmt.Fprintf(f, "url = %q\n", app.URL)
		}
		if app.Branch != "" {
			fmt.Fprintf(f, "branch = %q\n", app.Branch)
		}
		if app.Path != "" {
			fmt.Fprintf(f, "path = %q\n", app.Path)
		}
		if app.Excluded {
			fmt.Fprintf(f, "excluded = true\n")
		}
		fmt.Fprintf(f, "\n")
	}

	// Write sites
	for _, site := range cfg.Sites {
		fmt.Fprintf(f, "[[sites]]\n")
		fmt.Fprintf(f, "name = %q\n", site.Name)
		if site.DefaultSite {
			fmt.Fprintf(f, "default = true\n")
		}
		if len(site.Apps) > 0 {
			fmt.Fprintf(f, "apps = [")
			for i, app := range site.Apps {
				if i > 0 {
					fmt.Fprintf(f, ", ")
				}
				fmt.Fprintf(f, "%q", app)
			}
			fmt.Fprintf(f, "]\n")
		}
		fmt.Fprintf(f, "\n")
	}

	return nil
}
