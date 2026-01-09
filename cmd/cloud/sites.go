package cloud

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/gavindsouza/weg/internal/cloud"
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
		fmt.Println("No sites found")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tPLAN\tREGION\tCREATED")
	for _, site := range sites {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			site.Name,
			site.Status,
			site.Plan,
			site.Region,
			site.CreatedAt,
		)
	}
	w.Flush()

	return nil
}

// getAuthenticatedClient returns a cloud client for the specified cloud (or default)
func getAuthenticatedClient(cloudName string) (*cloud.Client, error) {
	return cloud.GetCloudClient(cloudName)
}
