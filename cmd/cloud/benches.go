package cloud

import (
	"fmt"

	"github.com/gavindsouza/weg/internal/output"
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
		output.Print("No benches found")
		return nil
	}

	type BenchRow struct {
		Name    string `json:"name"`
		Status  string `json:"status"`
		Version string `json:"version"`
		Apps    int    `json:"apps"`
		Sites   int    `json:"sites"`
	}

	var rows []BenchRow
	for _, bench := range benches {
		rows = append(rows, BenchRow{
			Name:    bench.Name,
			Status:  bench.Status,
			Version: bench.FrappeVersion,
			Apps:    bench.AppCount,
			Sites:   bench.SiteCount,
		})
	}

	return output.List(rows)
}
