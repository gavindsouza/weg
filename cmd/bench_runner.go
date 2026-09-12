package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/tools"
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

// needsFrappeGroup reports whether a bench command must be prefixed with
// bench_helper's "frappe" group (all frappe and app commands live under it):
// everything except "frappe" itself and bench_helper's get-frappe-* helpers.
func needsFrappeGroup(cmd string) bool {
	return cmd != "frappe" && !strings.HasPrefix(cmd, "get-frappe-")
}

// RunBench runs a bench command in the appropriate context
// For devbox projects, it runs via bench_helper from the sites directory
// Returns nil on success, an *exec.ExitError if the command ran and failed,
// any other error if it couldn't run in a weg project.
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
		} else if needsFrappeGroup(cmd) {
			// bench_helper only knows the "frappe" group (plus app groups):
			// `bench drop-site x` is `frappe drop-site x` to it.
			benchArgs = append([]string{"frappe"}, args...)
		} else {
			benchArgs = args
		}
	} else {
		benchArgs = args
	}

	execCmd := tools.FrappeCmd(benchPath, benchArgs...)
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	execCmd.Stdin = os.Stdin

	return execCmd.Run()
}
