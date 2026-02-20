package site

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gavindsouza/weg/internal/config"
	wegerrors "github.com/gavindsouza/weg/internal/errors"
	wegoutput "github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var siteConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage site configuration",
	Long: `View and modify site configuration values.

Configuration is stored in site_config.json for each site.

Examples:
  weg site config get db_name           # Get specific key
  weg site config get                   # Get all config
  weg site config set custom_key value  # Set a key
  weg site config set --json limits '{"limit":10}'`,
}

var configGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get configuration value(s)",
	Long: `Get site configuration values.

Without a key, shows all configuration.
With a key, shows only that specific value.

Examples:
  weg site config get              # Show all config
  weg site config get db_name      # Show db_name only
  weg site config get --site test  # From specific site`,
	Args: cobra.MaximumNArgs(1),
	RunE: runConfigGet,
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Long: `Set a site configuration value.

The value is automatically parsed as JSON if valid,
otherwise stored as a string.

Use --json flag for complex values.

Examples:
  weg site config set maintenance_mode 1
  weg site config set admin_password secret
  weg site config set --json limits '{"limit":10}'
  weg site config set db_host 127.0.0.1 --site test`,
	Args: cobra.ExactArgs(2),
	RunE: runConfigSet,
}

var configDeleteCmd = &cobra.Command{
	Use:   "delete <key>",
	Short: "Delete a configuration key",
	Args:  cobra.ExactArgs(1),
	RunE:  runConfigDelete,
}

var (
	configSite string
	configJSON bool
)

func init() {
	SiteCmd.AddCommand(siteConfigCmd)
	siteConfigCmd.AddCommand(configGetCmd)
	siteConfigCmd.AddCommand(configSetCmd)
	siteConfigCmd.AddCommand(configDeleteCmd)

	siteConfigCmd.PersistentFlags().StringVar(&configSite, "site", "", "Site to configure")
	configSetCmd.Flags().BoolVar(&configJSON, "json", false, "Parse value as JSON")
}

func resolveSiteConfig() (string, string, error) {
	path := "."
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", "", fmt.Errorf("invalid path: %w", err)
	}

	result, err := config.DetectProjectContext(absPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to detect context: %w", err)
	}

	var benchPath string
	switch result.Context {
	case config.ContextWegBench:
		benchPath = result.BenchPath
	case config.ContextWegApp:
		benchPath = result.BenchPath
	default:
		return "", "", wegerrors.NotInProject(absPath)
	}

	// Determine site
	site := configSite
	if site == "" {
		st, err := state.Load(absPath)
		if err == nil {
			site = st.GetDefaultSite()
		}
		if site == "" {
			currentSitePath := filepath.Join(benchPath, "sites", "currentsite.txt")
			data, _ := os.ReadFile(currentSitePath)
			site = strings.TrimSpace(string(data))
		}
	}

	if site == "" {
		return "", "", wegerrors.Usage("no site specified and no default site found")
	}

	return benchPath, site, nil
}

func loadSiteConfigJSON(benchPath, site string) (map[string]any, string, error) {
	configPath := filepath.Join(benchPath, "sites", site, "site_config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, configPath, wegerrors.Config("config", "read", err)
	}

	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, configPath, wegerrors.Config("config", "parse", err)
	}

	return cfg, configPath, nil
}

func saveSiteConfigJSON(path string, cfg map[string]any) error {
	data, err := json.MarshalIndent(cfg, "", " ")
	if err != nil {
		return wegerrors.Config("config", "write", err)
	}
	return os.WriteFile(path, data, 0600)
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	benchPath, site, err := resolveSiteConfig()
	if err != nil {
		return err
	}

	cfg, _, err := loadSiteConfigJSON(benchPath, site)
	if err != nil {
		return err
	}

	if len(args) == 0 {
		// Show all config
		output, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return err
		}
		wegoutput.Print(string(output))
	} else {
		// Show specific key
		key := args[0]
		if val, ok := cfg[key]; ok {
			switch v := val.(type) {
			case string:
				wegoutput.Print(v)
			case float64:
				if v == float64(int(v)) {
					wegoutput.Printf("%.0f", v)
				} else {
					wegoutput.Printf("%v", v)
				}
			case bool:
				wegoutput.Printf("%v", v)
			default:
				output, _ := json.MarshalIndent(v, "", "  ")
				wegoutput.Print(string(output))
			}
		} else {
			return wegerrors.NotFound("key", key)
		}
	}

	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	benchPath, site, err := resolveSiteConfig()
	if err != nil {
		return err
	}

	cfg, configPath, err := loadSiteConfigJSON(benchPath, site)
	if err != nil {
		return err
	}

	key := args[0]
	valueStr := args[1]

	var value any
	if configJSON {
		// Parse as JSON
		if err := json.Unmarshal([]byte(valueStr), &value); err != nil {
			return wegerrors.Validation("value", fmt.Sprintf("invalid JSON: %v", err))
		}
	} else {
		// Try to auto-detect type
		if valueStr == "true" {
			value = true
		} else if valueStr == "false" {
			value = false
		} else if i, err := strconv.Atoi(valueStr); err == nil {
			value = i
		} else if f, err := strconv.ParseFloat(valueStr, 64); err == nil {
			value = f
		} else if strings.HasPrefix(valueStr, "{") || strings.HasPrefix(valueStr, "[") {
			// Try parsing as JSON
			if err := json.Unmarshal([]byte(valueStr), &value); err == nil {
				// Successfully parsed as JSON
			} else {
				value = valueStr
			}
		} else {
			value = valueStr
		}
	}

	cfg[key] = value

	if err := saveSiteConfigJSON(configPath, cfg); err != nil {
		return err
	}

	wegoutput.Printf("Set %s = %v for %s", key, value, site)
	return nil
}

func runConfigDelete(cmd *cobra.Command, args []string) error {
	benchPath, site, err := resolveSiteConfig()
	if err != nil {
		return err
	}

	cfg, configPath, err := loadSiteConfigJSON(benchPath, site)
	if err != nil {
		return err
	}

	key := args[0]
	if _, ok := cfg[key]; !ok {
		return wegerrors.NotFound("key", key)
	}

	delete(cfg, key)

	if err := saveSiteConfigJSON(configPath, cfg); err != nil {
		return err
	}

	wegoutput.Printf("Deleted %s from %s", key, site)
	return nil
}
