package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/gavindsouza/weg/internal/config"
	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration value",
	Long: `Get a specific configuration value.

Keys use dot notation: section.key

Examples:
  weg config get frappe.version
  weg config get frappe.database
  weg config get apps.erpnext.branch`,
	Args:         cobra.ExactArgs(1),
	RunE:         runGet,
	SilenceUsage: true,
}

func runGet(cmd *cobra.Command, args []string) error {
	key := args[0]
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	result, err := config.DetectProjectContext(cwd)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	if result.Context != config.ContextWegBench && result.Context != config.ContextWegApp {
		return wegerrors.NotInProject(cwd)
	}

	value, err := getConfigValue(result, key)
	if err != nil {
		return err
	}

	output.Print(value)
	return nil
}

func getConfigValue(result *config.DetectionResult, key string) (string, error) {
	parts := strings.Split(key, ".")

	if result.Context == config.ContextWegBench {
		cfg, err := config.ParseWegToml(result.BenchPath)
		if err != nil {
			return "", err
		}
		return getValueFromBenchConfig(cfg, parts)
	}

	return "", wegerrors.NotFound("key", key)
}

func getValueFromBenchConfig(cfg *config.BenchConfig, parts []string) (string, error) {
	if len(parts) == 0 {
		return "", wegerrors.Validation("key", "must not be empty")
	}

	switch parts[0] {
	case "frappe":
		if len(parts) < 2 {
			return "", wegerrors.NotFound("key", "frappe")
		}
		switch parts[1] {
		case "version":
			return cfg.Frappe.Version, nil
		case "database":
			return cfg.Frappe.Database, nil
		}
	case "bench":
		if len(parts) < 2 {
			return "", wegerrors.NotFound("key", "bench")
		}
		switch parts[1] {
		case "name":
			return cfg.Bench.Name, nil
		}
	case "apps":
		if len(parts) < 3 {
			return "", wegerrors.Usage("usage: apps.<name>.<key>")
		}
		appName := parts[1]
		appCfg, ok := cfg.Apps[appName]
		if !ok {
			return "", wegerrors.NotFound("app", appName)
		}
		switch parts[2] {
		case "url":
			return appCfg.URL, nil
		case "branch":
			return appCfg.Branch, nil
		case "path":
			return appCfg.Path, nil
		}
	}

	return "", wegerrors.NotFound("key", strings.Join(parts, "."))
}
