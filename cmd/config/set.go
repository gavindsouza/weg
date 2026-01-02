package config

import (
	"fmt"
	"os"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/spf13/cobra"
)

var setCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Long: `Set a specific configuration value.

Keys use dot notation: section.key

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

	// For now, inform user to edit file manually
	// Full implementation would modify the TOML file
	fmt.Printf("To set %s = %q, please edit %s\n", key, value, result.ConfigPath)
	fmt.Println("\nNote: Automatic config editing coming soon.")

	return nil
}
