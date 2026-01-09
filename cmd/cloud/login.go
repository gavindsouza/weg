package cloud

import (
	"fmt"
	"strings"
	"syscall"

	"github.com/gavindsouza/weg/internal/cloud"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var loginCmd = &cobra.Command{
	Use:   "login [cloud-name]",
	Short: "Authenticate with Frappe Cloud",
	Long: `Log in to Frappe Cloud using your API key and secret.

You can get API credentials from https://cloud.frappe.io/dashboard/settings/api-access

Credentials are stored in:
  ~/.config/weg/credentials.toml  (global, with --global)
  .weg/credentials.toml           (local, default)

Examples:
  weg cloud login                                       # Login to default cloud (frappe)
  weg cloud login --global                              # Save credentials globally
  weg cloud login mycloud --url https://press.example.com  # Custom cloud instance
  weg cloud login --team myteam@example.com             # Specify team directly`,
	RunE: runLogin,
	Args: cobra.MaximumNArgs(1),
}

var (
	loginAPIKey    string
	loginAPISecret string
	loginCloudURL  string
	loginTeam      string
	loginGlobal    bool
)

func init() {
	loginCmd.Flags().StringVar(&loginAPIKey, "api-key", "", "API key")
	loginCmd.Flags().StringVar(&loginAPISecret, "api-secret", "", "API secret")
	loginCmd.Flags().StringVar(&loginCloudURL, "url", "", "Cloud URL (e.g., https://cloud.frappe.io)")
	loginCmd.Flags().StringVar(&loginTeam, "team", "", "Team to use (if you belong to multiple teams)")
	loginCmd.Flags().BoolVar(&loginGlobal, "global", false, "Save credentials globally (~/.weg/) instead of locally (.weg/)")
}

func runLogin(cmd *cobra.Command, args []string) error {
	// Cloud name (default: "frappe")
	cloudName := "frappe"
	if len(args) > 0 {
		cloudName = args[0]
	}

	apiKey := loginAPIKey
	apiSecret := loginAPISecret
	cloudURL := loginCloudURL
	team := loginTeam

	// Prompt for API key if not provided
	if apiKey == "" {
		fmt.Print("API key: ")
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

	// Prompt for API secret if not provided
	if apiSecret == "" {
		fmt.Print("API secret: ")
		secretBytes, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return fmt.Errorf("failed to read API secret: %w", err)
		}
		fmt.Println()
		apiSecret = string(secretBytes)
	}

	if apiSecret == "" {
		return fmt.Errorf("API secret is required")
	}

	// Combine key:secret for Frappe API token authentication
	token := apiKey + ":" + apiSecret

	// Build cloud URL with /api suffix if needed
	if cloudURL != "" {
		cloudURL = strings.TrimSuffix(cloudURL, "/")
		if !strings.HasSuffix(cloudURL, "/api") {
			cloudURL = cloudURL + "/api"
		}
	} else if cloudName == "frappe" {
		cloudURL = cloud.DefaultCloudAPI
	}

	// Verify the credentials work
	fmt.Println("Verifying credentials...")
	client := cloud.NewClientWithURL(token, cloudURL)

	user, err := client.GetCurrentUser()
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Handle team selection
	if team == "" {
		teams, err := client.GetTeams()
		if err != nil {
			team = user.Team
		} else if len(teams) == 0 {
			team = user.Team
		} else if len(teams) == 1 {
			team = teams[0].Name
		} else {
			// Multiple teams - prompt user to select
			fmt.Println("\nYou belong to multiple teams:")
			for i, t := range teams {
				status := ""
				if !t.Enabled {
					status = " (disabled)"
				}
				if t.Name == user.Team {
					fmt.Printf("  [%d] %s%s (current)\n", i+1, t.Name, status)
				} else {
					fmt.Printf("  [%d] %s%s\n", i+1, t.Name, status)
				}
			}
			fmt.Print("\nSelect team (number): ")
			var selection int
			if _, err := fmt.Scanf("%d", &selection); err != nil || selection < 1 || selection > len(teams) {
				fmt.Println("Using current team:", user.Team)
				team = user.Team
			} else {
				team = teams[selection-1].Name
			}
		}
	}

	// Load existing config and credentials
	config, _ := cloud.LoadConfig()
	if config == nil {
		config = &cloud.CloudConfig{Clouds: make(map[string]*cloud.CloudEntry)}
	}
	if config.Clouds == nil {
		config.Clouds = make(map[string]*cloud.CloudEntry)
	}

	creds, _ := cloud.LoadCredentials()
	if creds == nil {
		creds = &cloud.CloudCredentials{Clouds: make(map[string]*cloud.CloudAuth)}
	}
	if creds.Clouds == nil {
		creds.Clouds = make(map[string]*cloud.CloudAuth)
	}

	// Update config
	config.Clouds[cloudName] = &cloud.CloudEntry{
		URL:  cloudURL,
		Team: team,
	}
	if config.Default == "" {
		config.Default = cloudName
	}

	// Update credentials
	creds.Clouds[cloudName] = &cloud.CloudAuth{
		Token: token,
	}

	// Save
	if err := cloud.SaveConfig(config, loginGlobal); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	if err := cloud.SaveCredentials(creds, loginGlobal); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	// Success message
	scope := "locally (.weg/)"
	if loginGlobal {
		scope = "globally (~/.config/weg/)"
	}

	fmt.Printf("\n✓ Logged in as %s\n", user.Email)
	fmt.Printf("  Cloud: %s\n", cloudName)
	if team != "" {
		fmt.Printf("  Team: %s\n", team)
	}
	fmt.Printf("  Saved: %s\n", scope)
	return nil
}
