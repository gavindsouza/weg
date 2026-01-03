package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/spf13/cobra"
)

var benchConfigCmd = &cobra.Command{
	Use:   "bench-config",
	Short: "Manage common site configuration",
	Long: `View and modify the common_site_config.json file.

This configuration applies to all sites in the bench.

Examples:
  weg bench-config get                    # Show all config
  weg bench-config get redis_cache        # Get specific key
  weg bench-config set redis_cache redis://localhost:6379
  weg bench-config delete custom_key`,
}

var benchConfigGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get configuration value(s)",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runBenchConfigGet,
}

var benchConfigSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	RunE:  runBenchConfigSet,
}

var benchConfigDeleteCmd = &cobra.Command{
	Use:   "delete <key>",
	Short: "Delete a configuration key",
	Args:  cobra.ExactArgs(1),
	RunE:  runBenchConfigDelete,
}

var benchConfigJSON bool

func init() {
	rootCmd.AddCommand(benchConfigCmd)
	benchConfigCmd.AddCommand(benchConfigGetCmd)
	benchConfigCmd.AddCommand(benchConfigSetCmd)
	benchConfigCmd.AddCommand(benchConfigDeleteCmd)

	benchConfigSetCmd.Flags().BoolVar(&benchConfigJSON, "json", false, "Parse value as JSON")
}

func getBenchPath() (string, error) {
	path := "."
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	result, err := config.DetectContext(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to detect context: %w", err)
	}

	switch result.Context {
	case config.ContextWegBench:
		return absPath, nil
	case config.ContextWegApp:
		return filepath.Join(absPath, ".weg"), nil
	default:
		return "", fmt.Errorf("not a weg-managed project")
	}
}

func loadCommonConfig(benchPath string) (map[string]interface{}, string, error) {
	configPath := filepath.Join(benchPath, "sites", "common_site_config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]interface{}), configPath, nil
		}
		return nil, configPath, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, configPath, fmt.Errorf("failed to parse config: %w", err)
	}

	return cfg, configPath, nil
}

func saveCommonConfig(path string, cfg map[string]interface{}) error {
	data, err := json.MarshalIndent(cfg, "", " ")
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

func runBenchConfigGet(cmd *cobra.Command, args []string) error {
	benchPath, err := getBenchPath()
	if err != nil {
		return err
	}

	cfg, _, err := loadCommonConfig(benchPath)
	if err != nil {
		return err
	}

	if len(args) == 0 {
		output, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(output))
	} else {
		key := args[0]
		if val, ok := cfg[key]; ok {
			switch v := val.(type) {
			case string:
				fmt.Println(v)
			case float64:
				if v == float64(int(v)) {
					fmt.Printf("%.0f\n", v)
				} else {
					fmt.Printf("%v\n", v)
				}
			case bool:
				fmt.Printf("%v\n", v)
			default:
				output, _ := json.MarshalIndent(v, "", "  ")
				fmt.Println(string(output))
			}
		} else {
			return fmt.Errorf("key '%s' not found", key)
		}
	}

	return nil
}

func runBenchConfigSet(cmd *cobra.Command, args []string) error {
	benchPath, err := getBenchPath()
	if err != nil {
		return err
	}

	cfg, configPath, err := loadCommonConfig(benchPath)
	if err != nil {
		return err
	}

	key := args[0]
	valueStr := args[1]

	var value interface{}
	if benchConfigJSON {
		if err := json.Unmarshal([]byte(valueStr), &value); err != nil {
			return fmt.Errorf("invalid JSON value: %w", err)
		}
	} else {
		if valueStr == "true" {
			value = true
		} else if valueStr == "false" {
			value = false
		} else if i, err := strconv.Atoi(valueStr); err == nil {
			value = i
		} else if f, err := strconv.ParseFloat(valueStr, 64); err == nil {
			value = f
		} else if strings.HasPrefix(valueStr, "{") || strings.HasPrefix(valueStr, "[") {
			if err := json.Unmarshal([]byte(valueStr), &value); err == nil {
				// parsed as JSON
			} else {
				value = valueStr
			}
		} else {
			value = valueStr
		}
	}

	cfg[key] = value

	if err := saveCommonConfig(configPath, cfg); err != nil {
		return err
	}

	fmt.Printf("Set %s = %v\n", key, value)
	return nil
}

func runBenchConfigDelete(cmd *cobra.Command, args []string) error {
	benchPath, err := getBenchPath()
	if err != nil {
		return err
	}

	cfg, configPath, err := loadCommonConfig(benchPath)
	if err != nil {
		return err
	}

	key := args[0]
	if _, ok := cfg[key]; !ok {
		return fmt.Errorf("key '%s' not found", key)
	}

	delete(cfg, key)

	if err := saveCommonConfig(configPath, cfg); err != nil {
		return err
	}

	fmt.Printf("Deleted %s\n", key)
	return nil
}
