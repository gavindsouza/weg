package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/errors"
)

// getDefaultSite returns the default site from config. benchPath already
// points at the config directory in both contexts (.weg/ for app-centric,
// the bench root for bench-centric).
func getDefaultSite(benchPath string, result *config.DetectionResult) string {
	if cfg, err := config.ParseWegToml(benchPath); err == nil {
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

// Commands that should get "frappe" prefix (no site needed)
var frappeCommands = map[string]bool{
	"build": true, "watch": true, "version": true,
}

// RunBench runs a bench command in the appropriate context
// For devbox projects, it runs via bench_helper from the sites directory
// Returns error if it couldn't run, nil if it ran (even if command failed)
func RunBench(args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	result, err := config.DetectProjectContext(cwd)
	if err != nil {
		return err
	}

	var benchPath string
	switch result.Context {
	case config.ContextWegBench:
		benchPath = result.BenchPath
	case config.ContextWegApp:
		benchPath = filepath.Join(cwd, ".weg")
	default:
		return errors.NotInProject(cwd)
	}

	// Check for devbox
	devboxJSON := filepath.Join(benchPath, "devbox.json")
	if _, err := os.Stat(devboxJSON); err != nil {
		return fmt.Errorf("no devbox.json found")
	}

	// Get default site from config
	site := getDefaultSite(benchPath, result)

	// Build command args - add appropriate prefix
	var benchArgs []string
	if len(args) > 0 {
		cmd := args[0]
		if siteCommands[cmd] && site != "" {
			// Site-specific commands: "frappe --site <site> <cmd>"
			benchArgs = append([]string{"frappe", "--site", site}, args...)
		} else if frappeCommands[cmd] {
			// Frappe commands without site: "frappe <cmd>"
			benchArgs = append([]string{"frappe"}, args...)
		} else {
			benchArgs = args
		}
	} else {
		benchArgs = args
	}

	// Run via devbox from sites directory
	sitesDir := filepath.Join(benchPath, "sites")
	pythonPath := filepath.Join(benchPath, "env", "bin", "python")
	devboxArgs := []string{"run", "-c", benchPath, "--", pythonPath, "-m", "frappe.utils.bench_helper"}
	devboxArgs = append(devboxArgs, benchArgs...)

	execCmd := exec.Command("devbox", devboxArgs...)
	execCmd.Dir = sitesDir
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	execCmd.Stdin = os.Stdin
	execCmd.Env = os.Environ()

	return execCmd.Run()
}
