package cloud

import (
	"fmt"

	"github.com/gavindsouza/weg/internal/cloud"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/spf13/cobra"
)

var sitesCmd = &cobra.Command{
	Use:   "sites",
	Short: "List your Frappe Cloud sites",
	Long: `List all sites on your Frappe Cloud account.

Examples:
  weg cloud sites                # List all sites
  weg cloud sites --team myteam  # List sites for a team
  weg cloud sites --cloud mycloud # Use specific cloud`,
	RunE: runSites,
}

var (
	sitesTeam  string
	sitesCloud string
)

func init() {
	sitesCmd.Flags().StringVar(&sitesTeam, "team", "", "Filter by team name")
	sitesCmd.Flags().StringVar(&sitesCloud, "cloud", "", "Which cloud to use (default: from config)")
}

func runSites(cmd *cobra.Command, args []string) error {
	client, err := getAuthenticatedClient(sitesCloud)
	if err != nil {
		return err
	}

	sites, err := client.ListSites(sitesTeam)
	if err != nil {
		return fmt.Errorf("failed to list sites: %w", err)
	}

	if len(sites) == 0 {
		output.Print("No sites found")
		return nil
	}

	type SiteRow struct {
		Name    string `json:"name"`
		Status  string `json:"status"`
		Plan    string `json:"plan"`
		Region  string `json:"region"`
		Created string `json:"created"`
	}

	var rows []SiteRow
	for _, site := range sites {
		rows = append(rows, SiteRow{
			Name:    site.Name,
			Status:  site.Status,
			Plan:    site.Plan,
			Region:  site.Region,
			Created: site.CreatedAt,
		})
	}

	return output.List(rows)
}

// getAuthenticatedClient returns a cloud client for the specified cloud (or default)
func getAuthenticatedClient(cloudName string) (*cloud.Client, error) {
	return cloud.GetCloudClient(cloudName)
}
