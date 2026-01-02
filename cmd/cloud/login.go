package cloud

import (
	"fmt"
	"os"
	"syscall"

	"github.com/gavindsouza/weg/internal/cloud"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Frappe Cloud",
	Long: `Log in to Frappe Cloud using your API key.

You can get an API key from https://frappecloud.com/dashboard/settings/api-access

Examples:
  weg cloud login                    # Interactive login
  weg cloud login --api-key <key>    # Non-interactive`,
	RunE: runLogin,
}

var loginAPIKey string

func init() {
	loginCmd.Flags().StringVar(&loginAPIKey, "api-key", "", "Frappe Cloud API key")
}

func runLogin(cmd *cobra.Command, args []string) error {
	apiKey := loginAPIKey

	if apiKey == "" {
		fmt.Print("Enter Frappe Cloud API key: ")
		keyBytes, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return fmt.Errorf("failed to read API key: %w", err)
		}
		fmt.Println()
		apiKey = string(keyBytes)
	}

	if apiKey == "" {
		return fmt.Errorf("API key is required")
	}

	// Verify the API key works
	fmt.Println("Verifying credentials...")
	client := cloud.NewClient(apiKey)

	user, err := client.GetCurrentUser()
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Save credentials
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	if err := cloud.SaveCredentials(homeDir, apiKey); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	fmt.Printf("Logged in as %s\n", user.Email)
	return nil
}
