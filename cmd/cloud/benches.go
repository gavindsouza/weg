package cloud

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var benchesCmd = &cobra.Command{
	Use:   "benches",
	Short: "List your Frappe Cloud benches",
	Long: `List all benches on your Frappe Cloud account.

Examples:
  weg cloud benches                # List all benches
  weg cloud benches --team myteam  # List benches for a team`,
	RunE:         runBenches,
	SilenceUsage: true,
}

var benchesTeam string

func init() {
	benchesCmd.Flags().StringVar(&benchesTeam, "team", "", "Filter by team name")
}

func runBenches(cmd *cobra.Command, args []string) error {
	client, err := getAuthenticatedClient("")
	if err != nil {
		return err
	}

	benches, err := client.ListBenches(benchesTeam)
	if err != nil {
		return fmt.Errorf("failed to list benches: %w", err)
	}

	if len(benches) == 0 {
		fmt.Println("No benches found")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tVERSION\tAPPS\tSITES")
	for _, bench := range benches {
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%d\n",
			bench.Name,
			bench.Status,
			bench.FrappeVersion,
			bench.AppCount,
			bench.SiteCount,
		)
	}
	w.Flush()

	return nil
}
