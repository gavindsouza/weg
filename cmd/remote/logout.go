/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package remote

import (
	"fmt"
	"net/url"

	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/remote"
	"github.com/spf13/cobra"
)

var logoutCmd = &cobra.Command{
	Use:   "logout <url>",
	Short: "Remove saved credentials for a remote site",
	Long: `Remove saved API credentials for a remote Frappe site.

This removes the credentials from ~/.config/weg/credentials.toml.

Examples:
  weg remote logout https://mysite.frappe.cloud`,
	Args: cobra.ExactArgs(1),
	RunE: runLogout,
}

func runLogout(cobraCmd *cobra.Command, args []string) error {
	siteURL := args[0]

	// Parse URL
	parsedURL, err := url.Parse(siteURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	siteHost := parsedURL.Hostname()
	if siteHost == "" {
		// Maybe they just passed the hostname
		siteHost = siteURL
	}

	// Check if credentials exist
	if !remote.HasGlobalCredentials(siteHost) {
		output.Printf("No saved credentials found for %s", siteHost)
		return nil
	}

	// Remove credentials
	if err := remote.RemoveGlobalCredentials(siteHost); err != nil {
		return fmt.Errorf("failed to remove credentials: %w", err)
	}

	globalDir, _ := remote.GlobalConfigDir()
	output.Printf("Credentials removed for %s", siteHost)
	output.Printf("  (stored at %s/credentials.toml)", globalDir)

	return nil
}
