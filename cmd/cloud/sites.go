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
  weg cloud sites --team myteam  # List sites for a team`,
	RunE: runSites,
}

var sitesTeam string

func init() {
	sitesCmd.Flags().StringVar(&sitesTeam, "team", "", "Filter by team name")
}

func runSites(cmd *cobra.Command, args []string) error {
	client, err := getAuthenticatedClient()
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

func getAuthenticatedClient() (*cloud.Client, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	apiKey, err := cloud.LoadCredentials(homeDir)
	if err != nil {
		return nil, fmt.Errorf("not logged in. Run 'weg cloud login' first")
	}

	return cloud.NewClient(apiKey), nil
}
