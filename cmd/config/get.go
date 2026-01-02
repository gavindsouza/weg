package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/gavindsouza/weg/internal/config"
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

	result, err := config.DetectContext(cwd)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	if result.Context != config.ContextWegBench && result.Context != config.ContextWegApp {
		return fmt.Errorf("not a weg-managed project")
	}

	value, err := getConfigValue(result, key)
	if err != nil {
		return err
	}

	fmt.Println(value)
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

	return "", fmt.Errorf("key not found: %s", key)
}

func getValueFromBenchConfig(cfg *config.BenchConfig, parts []string) (string, error) {
	if len(parts) == 0 {
		return "", fmt.Errorf("empty key")
	}

	switch parts[0] {
	case "frappe":
		if len(parts) < 2 {
			return "", fmt.Errorf("missing frappe key")
		}
		switch parts[1] {
		case "version":
			return cfg.Frappe.Version, nil
		case "database":
			return cfg.Frappe.Database, nil
		}
	case "bench":
		if len(parts) < 2 {
			return "", fmt.Errorf("missing bench key")
		}
		switch parts[1] {
		case "name":
			return cfg.Bench.Name, nil
		}
	case "apps":
		if len(parts) < 3 {
			return "", fmt.Errorf("usage: apps.<name>.<key>")
		}
		appName := parts[1]
		appCfg, ok := cfg.Apps[appName]
		if !ok {
			return "", fmt.Errorf("app not found: %s", appName)
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

	return "", fmt.Errorf("key not found: %s", strings.Join(parts, "."))
}
