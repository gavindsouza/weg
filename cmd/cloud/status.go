package cloud

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/gavindsouza/weg/internal/cloud"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status [deployment-id]",
	Short: "Check deployment status",
	Long: `Check the status of a deployment or site.

Examples:
  weg cloud status                    # Status of recent deployments
  weg cloud status <deployment-id>    # Status of specific deployment
  weg cloud status --site mysite      # Status of a site
  weg cloud status --watch            # Watch deployment progress`,
	RunE: runStatus,
}

var (
	statusSite  string
	statusWatch bool
)

func init() {
	statusCmd.Flags().StringVar(&statusSite, "site", "", "Check status of a specific site")
	statusCmd.Flags().BoolVarP(&statusWatch, "watch", "w", false, "Watch for updates")
}

func runStatus(cmd *cobra.Command, args []string) error {
	client, err := getAuthenticatedClient()
	if err != nil {
		return err
	}

	// Check specific deployment
	if len(args) > 0 {
		deployID := args[0]
		return watchDeployment(client, deployID, statusWatch)
	}

	// Check specific site
	if statusSite != "" {
		site, err := client.GetSite(statusSite)
		if err != nil {
			return fmt.Errorf("failed to get site: %w", err)
		}

		fmt.Printf("Site: %s\n", site.Name)
		fmt.Printf("Status: %s\n", site.Status)
		fmt.Printf("Plan: %s\n", site.Plan)
		fmt.Printf("Region: %s\n", site.Region)
		fmt.Printf("Created: %s\n", site.CreatedAt)
		return nil
	}

	// List recent deployments
	deploys, err := client.ListDeployments("")
	if err != nil {
		return fmt.Errorf("failed to list deployments: %w", err)
	}

	if len(deploys) == 0 {
		fmt.Println("No recent deployments")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSITE\tSTATUS\tSTARTED\tDURATION")
	for _, d := range deploys {
		duration := ""
		if d.FinishedAt != "" {
			duration = d.Duration
		} else if d.StartedAt != "" {
			duration = "running..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			d.ID,
			d.Site,
			d.Status,
			d.StartedAt,
			duration,
		)
	}
	w.Flush()

	return nil
}

func watchDeployment(client *cloud.Client, deployID string, watch bool) error {
	for {
		deploy, err := client.GetDeployment(deployID)
		if err != nil {
			return fmt.Errorf("failed to get deployment: %w", err)
		}

		fmt.Printf("\033[2J\033[H") // Clear screen
		fmt.Printf("Deployment: %s\n", deploy.ID)
		fmt.Printf("Site: %s\n", deploy.Site)
		fmt.Printf("Status: %s\n", deploy.Status)
		fmt.Printf("Started: %s\n", deploy.StartedAt)

		if deploy.Status == "Success" || deploy.Status == "Failed" {
			fmt.Printf("Finished: %s\n", deploy.FinishedAt)
			fmt.Printf("Duration: %s\n", deploy.Duration)
			if deploy.Status == "Failed" {
				fmt.Printf("\nError: %s\n", deploy.Error)
			}
			return nil
		}

		if !watch {
			return nil
		}

		time.Sleep(2 * time.Second)
	}
}
