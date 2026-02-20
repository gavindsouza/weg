/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package remote

import (
	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/output"
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
		return wegerrors.NotFound("remote clone", ".weg/site.toml")
	}

	// Load config
	config, err := remote.LoadSiteConfig(".")
	if err != nil {
		return wegerrors.Config("site.toml", "read", err)
	}

	// Site info
	output.Print("Site Information")
	output.Print("================")
	output.Printf("  URL:      %s", config.Site.URL)
	output.Printf("  Name:     %s", config.Site.Name)
	output.Printf("  Cloned:   %s", config.Site.ClonedAt.Format("2006-01-02 15:04:05"))
	output.Printf("  Frappe:   %s", config.Site.Frappe.Version)
	output.Print("")

	// Apps
	if len(config.Site.Apps) > 0 {
		output.Print("Installed Apps")
		output.Print("--------------")
		for name, app := range config.Site.Apps {
			output.Printf("  %-20s %s", name, app.Version)
		}
		output.Print("")
	}

	// Modules
	if len(config.Modules) > 0 {
		output.Print("Modules")
		output.Print("-------")
		for name, mod := range config.Modules {
			syncStatus := "+"
			if !mod.Sync {
				syncStatus = "-"
			}
			output.Printf("  %s %-20s (app: %s)", syncStatus, name, mod.App)
		}
		output.Print("")
	}

	// Sync settings
	output.Print("Sync Configuration")
	output.Print("------------------")
	output.Printf("  Last sync: %s", config.Sync.LastSync.Format("2006-01-02 15:04:05"))
	output.Print("")
	output.Print("  Entity types:")
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
		output.Print("")
		output.Print("  Exclusion patterns:")
		for _, p := range config.Sync.Exclude.Patterns {
			output.Printf("    - %s", p)
		}
	}

	return nil
}

func printEntityStatus(name string, enabled bool) {
	status := "+"
	if !enabled {
		status = "-"
	}
	output.Printf("  %s %s", status, name)
}
