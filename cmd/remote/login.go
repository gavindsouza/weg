/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package remote

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/remote"
	"github.com/spf13/cobra"
)

var (
	loginAPIKey    string
	loginAPISecret string
)

var loginCmd = &cobra.Command{
	Use:   "login <url>",
	Short: "Save credentials for a remote site",
	Long: `Save API credentials for a remote Frappe site globally.

Credentials are stored in ~/.config/weg/credentials.toml and will be
automatically used when cloning or syncing with this site.

The credentials file is created with restricted permissions (0600).

Examples:
  weg remote login https://mysite.frappe.cloud
  weg remote login https://mysite.frappe.cloud --api-key=KEY --api-secret=SECRET`,
	Args: cobra.ExactArgs(1),
	RunE: runLogin,
}

func init() {
	loginCmd.Flags().StringVar(&loginAPIKey, "api-key", "", "API key")
	loginCmd.Flags().StringVar(&loginAPISecret, "api-secret", "", "API secret")
}

func runLogin(cobraCmd *cobra.Command, args []string) error {
	siteURL := args[0]

	// Parse URL
	parsedURL, err := url.Parse(siteURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Ensure scheme is set; use http for localhost
	isLocalhost := strings.Contains(parsedURL.Hostname(), "localhost") ||
		parsedURL.Hostname() == "127.0.0.1" ||
		strings.HasSuffix(parsedURL.Hostname(), ".local")

	if parsedURL.Scheme == "" {
		if isLocalhost {
			parsedURL.Scheme = "http"
		} else {
			parsedURL.Scheme = "https"
		}
		siteURL = parsedURL.String()
	}

	siteHost := parsedURL.Hostname()

	// Get credentials
	apiKey := loginAPIKey
	apiSecret := loginAPISecret

	// Try environment variables
	if apiKey == "" {
		apiKey = os.Getenv("WEG_API_KEY")
	}
	if apiSecret == "" {
		apiSecret = os.Getenv("WEG_API_SECRET")
	}

	// Interactive prompt if needed
	if apiKey == "" || apiSecret == "" {
		fmt.Println("Enter API credentials for", siteHost)
		fmt.Println()
		fmt.Println("To create API credentials on your Frappe site:")
		fmt.Println("  1. Go to User Settings > API Access")
		fmt.Println("  2. Generate a new API Key + Secret")
		fmt.Println()

		reader := bufio.NewReader(os.Stdin)

		if apiKey == "" {
			fmt.Print("API Key: ")
			apiKey, _ = reader.ReadString('\n')
			apiKey = strings.TrimSpace(apiKey)
		}

		if apiSecret == "" {
			fmt.Print("API Secret: ")
			apiSecret, _ = reader.ReadString('\n')
			apiSecret = strings.TrimSpace(apiSecret)
		}
	}

	if apiKey == "" || apiSecret == "" {
		return fmt.Errorf("API key and secret are required")
	}

	// Test connection
	output.Infof("Testing connection to %s...\n", siteURL)
	client := remote.NewClient(siteURL, apiKey, apiSecret)
	if err := client.Ping(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	fmt.Println("Connected")

	// Save credentials
	if err := remote.SaveGlobalCredentials(siteHost, &remote.CredentialAuth{
		APIKey:    apiKey,
		APISecret: apiSecret,
	}); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	fmt.Printf("Credentials saved for %s\n", siteHost)
	fmt.Println()
	fmt.Println("You can now clone this site without entering credentials:")
	fmt.Printf("  weg remote clone %s\n", siteURL)

	return nil
}
