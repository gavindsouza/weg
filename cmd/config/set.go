package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/spf13/cobra"
)

var setCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Long: `Set a specific configuration value.

Keys use dot notation: section.key

Supported keys for weg.toml:
  frappe.version    - Frappe version (14, 15, 16)
  frappe.database   - Database type (mariadb, postgres, sqlite)
  bench.name        - Bench name

Examples:
  weg config set frappe.version 15
  weg config set frappe.database postgres`,
	Args:         cobra.ExactArgs(2),
	RunE:         runSet,
	SilenceUsage: true,
}

func runSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]

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

	// Only support weg.toml for now (pyproject.toml editing is more complex)
	if result.Context == config.ContextWegApp {
		output.Warning("Config editing for pyproject.toml not yet supported")
		fmt.Printf("To set %s = %q, please edit %s manually\n", key, value, result.ConfigPath)
		return nil
	}

	// Read the current config file
	data, err := os.ReadFile(result.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	content := string(data)
	var updated string
	var found bool

	switch key {
	case "frappe.version":
		updated, found = updateTOMLValue(content, "frappe", "version", value)
	case "frappe.database":
		updated, found = updateTOMLValue(content, "frappe", "database", value)
	case "bench.name":
		updated, found = updateTOMLValue(content, "bench", "name", value)
	default:
		output.Warningf("Key %q not supported for automatic editing", key)
		fmt.Printf("To set %s = %q, please edit %s manually\n", key, value, result.ConfigPath)
		return nil
	}

	if !found {
		return fmt.Errorf("key %q not found in config file", key)
	}

	// Write back
	if err := os.WriteFile(result.ConfigPath, []byte(updated), 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	output.Successf("Set %s = %q", key, value)
	return nil
}

// updateTOMLValue updates a value in a TOML file while preserving formatting
// Returns the updated content and whether the key was found
func updateTOMLValue(content, section, key, value string) (string, bool) {
	// Build regex to find the section and key
	// This handles: [section] ... key = "value" or key = value
	lines := strings.Split(content, "\n")
	inSection := false
	found := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for section header
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			sectionName := strings.Trim(trimmed, "[]")
			inSection = (sectionName == section)
			continue
		}

		// Check for key in current section
		if inSection && strings.HasPrefix(trimmed, key) {
			// Match: key = value or key = "value"
			pattern := regexp.MustCompile(`^(\s*)` + regexp.QuoteMeta(key) + `\s*=\s*(.*)$`)
			if pattern.MatchString(line) {
				// Preserve leading whitespace
				indent := ""
				if idx := strings.Index(line, key); idx > 0 {
					indent = line[:idx]
				}
				// Quote string values, leave numbers unquoted
				if _, err := fmt.Sscanf(value, "%d", new(int)); err != nil {
					// Not a number, quote it
					lines[i] = fmt.Sprintf("%s%s = %q", indent, key, value)
				} else {
					lines[i] = fmt.Sprintf("%s%s = %s", indent, key, value)
				}
				found = true
				break
			}
		}
	}

	return strings.Join(lines, "\n"), found
}
