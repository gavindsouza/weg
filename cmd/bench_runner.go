package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/config"
)

// getDefaultSite returns the default site from config
func getDefaultSite(benchPath string, result *config.DetectionResult) string {
	// For app-centric projects, check .weg/weg.toml
	var configPath string
	if result.Context == config.ContextWegApp {
		configPath = filepath.Join(benchPath, "weg.toml")
	} else {
		configPath = filepath.Join(benchPath, "weg.toml")
	}

	// Try to get from bench config
	if cfg, err := config.ParseWegToml(filepath.Dir(configPath)); err == nil {
		for _, site := range cfg.Sites {
			if site.DefaultSite {
				return site.Name
			}
		}
		// If no default, return first site
		if len(cfg.Sites) > 0 {
			return cfg.Sites[0].Name
		}
	}

	// Fallback: look for any site directory
	sitesDir := filepath.Join(benchPath, "sites")
	entries, err := os.ReadDir(sitesDir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") && entry.Name() != "assets" {
			return entry.Name()
		}
	}
	return ""
}

// Commands that should automatically get "frappe --site <site>" prefix
var siteCommands = map[string]bool{
	"migrate": true, "console": true, "mariadb": true, "db-console": true,
	"backup": true, "restore": true, "set-config": true, "clear-cache": true,
	"scheduler": true, "execute": true, "install-app": true, "uninstall-app": true,
	"list-apps": true, "add-user": true, "disable-user": true, "set-password": true,
	"browse": true,
}

// RunBench runs a bench command in the appropriate context
// For devbox projects, it runs via bench_helper from the sites directory
// Returns error if it couldn't run, nil if it ran (even if command failed)
func RunBench(args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	result, err := config.DetectContext(cwd)
	if err != nil {
		return err
	}

	var benchPath string
	switch result.Context {
	case config.ContextWegBench:
		benchPath = cwd
	case config.ContextWegApp:
		benchPath = filepath.Join(cwd, ".weg")
	default:
		return fmt.Errorf("not a weg-managed project")
	}

	// Check for devbox
	devboxJSON := filepath.Join(benchPath, "devbox.json")
	if _, err := os.Stat(devboxJSON); err != nil {
		return fmt.Errorf("no devbox.json found")
	}

	// Get default site from config
	site := getDefaultSite(benchPath, result)

	// Build command args - add "frappe --site <site>" prefix for site commands
	var benchArgs []string
	if len(args) > 0 && siteCommands[args[0]] && site != "" {
		benchArgs = append([]string{"frappe", "--site", site}, args...)
	} else {
		benchArgs = args
	}

	// Run via devbox from sites directory
	sitesDir := filepath.Join(benchPath, "sites")
	shellCmd := fmt.Sprintf("cd %s && ../.venv/bin/python -m frappe.utils.bench_helper %s",
		sitesDir, strings.Join(benchArgs, " "))

	execCmd := exec.Command("devbox", "run", "-c", benchPath, "--", "sh", "-c", shellCmd)
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	execCmd.Stdin = os.Stdin
	execCmd.Env = os.Environ()

	return execCmd.Run()
}
