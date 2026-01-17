/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package remote

import (
	"fmt"

	"github.com/gavindsouza/weg/internal/remote"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show information about the remote site",
	Long: `Display information about the cloned remote site.

Shows:
  - Site URL and name
  - Frappe version
  - Installed apps and versions
  - Sync configuration
  - Last sync timestamp`,
	RunE: runInfo,
}

func runInfo(cobraCmd *cobra.Command, args []string) error {
	// Check if we're in a remote site directory
	if !remote.IsRemoteSite(".") {
		return fmt.Errorf("not a remote site clone (no .weg/site.toml found)")
	}

	// Load config
	config, err := remote.LoadSiteConfig(".")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Site info
	fmt.Println("Site Information")
	fmt.Println("================")
	fmt.Printf("  URL:      %s\n", config.Site.URL)
	fmt.Printf("  Name:     %s\n", config.Site.Name)
	fmt.Printf("  Cloned:   %s\n", config.Site.ClonedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("  Frappe:   %s\n", config.Site.Frappe.Version)
	fmt.Println()

	// Apps
	if len(config.Site.Apps) > 0 {
		fmt.Println("Installed Apps")
		fmt.Println("--------------")
		for name, app := range config.Site.Apps {
			fmt.Printf("  %-20s %s\n", name, app.Version)
		}
		fmt.Println()
	}

	// Modules
	if len(config.Modules) > 0 {
		fmt.Println("Modules")
		fmt.Println("-------")
		for name, mod := range config.Modules {
			syncStatus := "✓"
			if !mod.Sync {
				syncStatus = "✗"
			}
			fmt.Printf("  %s %-20s (app: %s)\n", syncStatus, name, mod.App)
		}
		fmt.Println()
	}

	// Sync settings
	fmt.Println("Sync Configuration")
	fmt.Println("------------------")
	fmt.Printf("  Last sync: %s\n", config.Sync.LastSync.Format("2006-01-02 15:04:05"))
	fmt.Println()
	fmt.Println("  Entity types:")
	printEntityStatus("    DocTypes", config.Sync.Entities.DocType)
	printEntityStatus("    Custom Fields", config.Sync.Entities.CustomField)
	printEntityStatus("    Property Setters", config.Sync.Entities.PropertySetter)
	printEntityStatus("    Client Scripts", config.Sync.Entities.ClientScript)
	printEntityStatus("    Server Scripts", config.Sync.Entities.ServerScript)
	printEntityStatus("    Reports", config.Sync.Entities.Report)
	printEntityStatus("    Print Formats", config.Sync.Entities.PrintFormat)
	printEntityStatus("    Workflows", config.Sync.Entities.Workflow)
	printEntityStatus("    Notifications", config.Sync.Entities.Notification)
	printEntityStatus("    Letter Heads", config.Sync.Entities.LetterHead)

	// Exclusions
	if len(config.Sync.Exclude.Patterns) > 0 {
		fmt.Println()
		fmt.Println("  Exclusion patterns:")
		for _, p := range config.Sync.Exclude.Patterns {
			fmt.Printf("    - %s\n", p)
		}
	}

	return nil
}

func printEntityStatus(name string, enabled bool) {
	status := "✓"
	if !enabled {
		status = "✗"
	}
	fmt.Printf("  %s %s\n", status, name)
}
