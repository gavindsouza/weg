package cloud

import (
	"fmt"
	"time"

	"github.com/gavindsouza/weg/internal/cloud"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status [job-id]",
	Short: "Check cloud account, site, or job status",
	Long: `Check the status of your Frappe Cloud resources.

Without arguments, shows account overview (benches and sites).
With a job ID, tracks that specific job.

Examples:
  weg cloud status                    # Account overview
  weg cloud status --site mysite      # Detailed site status
  weg cloud status --bench mybench    # Bench status and jobs
  weg cloud status <job-id>           # Track specific job
  weg cloud status <job-id> --watch   # Watch job progress`,
	RunE: runStatus,
}

var (
	statusSite  string
	statusBench string
	statusWatch bool
)

func init() {
	statusCmd.Flags().StringVar(&statusSite, "site", "", "Check status of a specific site")
	statusCmd.Flags().StringVar(&statusBench, "bench", "", "Check status of a specific bench")
	statusCmd.Flags().BoolVarP(&statusWatch, "watch", "w", false, "Watch for updates (with job ID)")
}

func runStatus(cmd *cobra.Command, args []string) error {
	client, err := getAuthenticatedClient("")
	if err != nil {
		return err
	}

	// Track specific job
	if len(args) > 0 {
		jobID := args[0]
		return trackJob(client, jobID, statusWatch)
	}

	// Show specific site status
	if statusSite != "" {
		return showSiteStatus(client, statusSite)
	}

	// Show specific bench status
	if statusBench != "" {
		return showBenchStatus(client, statusBench)
	}

	// Default: show account overview
	return showAccountOverview(client)
}

func showAccountOverview(client *cloud.Client) error {
	// Get user info
	user, err := client.GetCurrentUser()
	if err != nil {
		return fmt.Errorf("failed to get user info: %w", err)
	}

	if user.Name != "" {
		fmt.Printf("Account: %s (%s)\n", user.Name, user.Email)
	} else {
		fmt.Printf("Account: %s\n", user.Email)
	}
	if user.Team != "" {
		fmt.Printf("Team: %s\n", user.Team)
	}
	fmt.Println()

	// Get benches
	benches, err := client.ListBenches("")
	if err == nil && len(benches) > 0 {
		fmt.Printf("Benches (%d):\n", len(benches))
		t := output.NewTable("Name", "Version", "Sites", "Apps", "Status")
		for _, b := range benches {
			t.Row(b.Name, b.FrappeVersion, b.SiteCount, b.AppCount, b.Status)
		}
		t.Flush()
		fmt.Println()
	}

	// Get sites
	sites, err := client.ListSites("")
	if err == nil && len(sites) > 0 {
		fmt.Printf("Sites (%d):\n", len(sites))
		t := output.NewTable("Name", "Status", "Plan", "Region")
		for _, s := range sites {
			t.Row(s.Name, s.Status, s.Plan, s.Region)
		}
		t.Flush()
		fmt.Println()
	}

	if (benches == nil || len(benches) == 0) && (sites == nil || len(sites) == 0) {
		fmt.Println("No benches or sites found.")
		fmt.Println("\nYou may only have marketplace apps. Use 'weg cloud mp' to check.")
	}

	return nil
}

func showSiteStatus(client *cloud.Client, siteName string) error {
	site, err := client.GetSiteDetail(siteName)
	if err != nil {
		return fmt.Errorf("failed to get site: %w", err)
	}

	fmt.Printf("Site: %s\n", site.Name)
	fmt.Printf("Status: %s\n", site.Status)
	if site.BenchTitle != "" {
		fmt.Printf("Bench: %s\n", site.BenchTitle)
	}
	if site.FrappeVersion != "" {
		fmt.Printf("Frappe: %s\n", site.FrappeVersion)
	}
	fmt.Printf("Created: %s\n", site.CreatedAt)

	if site.UpdateAvailable {
		fmt.Println("\n[!] Updates available")
	}

	// Show installed apps
	if len(site.InstalledApps) > 0 {
		fmt.Println("\nInstalled Apps:")
		t := output.NewTable("App", "Branch", "Commit")
		for _, app := range site.InstalledApps {
			hash := app.Hash
			if len(hash) > 7 {
				hash = hash[:7]
			}
			t.Row(app.App, app.Branch, hash)
		}
		t.Flush()
	}

	// Show running jobs
	runningJobs, err := client.GetRunningJobs(siteName)
	if err == nil && len(runningJobs) > 0 {
		fmt.Println("\nRunning Jobs:")
		for _, job := range runningJobs {
			fmt.Printf("  - %s (%s) started %s\n", job.JobType, job.Name, job.Start)
		}
	}

	// Show recent jobs
	recentJobs, err := client.GetSiteJobs(siteName, 5)
	if err == nil && len(recentJobs) > 0 {
		fmt.Println("\nRecent Jobs:")
		t := output.NewTable("Type", "Status", "Duration", "Started")
		for _, job := range recentJobs {
			t.Row(job.JobType, job.Status, job.Duration, job.Creation)
		}
		t.Flush()
	}

	return nil
}

func showBenchStatus(client *cloud.Client, benchName string) error {
	// Get bench jobs
	jobs, err := client.GetBenchJobs(benchName, 10)
	if err != nil {
		return fmt.Errorf("failed to get bench info: %w", err)
	}

	fmt.Printf("Bench: %s\n", benchName)
	fmt.Println("\nRecent Jobs:")

	if len(jobs) == 0 {
		fmt.Println("  No recent jobs")
		return nil
	}

	t := output.NewTable("Job", "Type", "Status", "Duration", "Started")
	for _, job := range jobs {
		name := job.Name
		if len(name) > 15 {
			name = name[:15] + "..."
		}
		t.Row(name, job.JobType, job.Status, job.Duration, job.Creation)
	}
	t.Flush()

	return nil
}

func trackJob(client *cloud.Client, jobID string, watch bool) error {
	for {
		job, err := client.GetJob(jobID)
		if err != nil {
			return fmt.Errorf("failed to get job: %w", err)
		}

		if watch {
			// Move cursor to top (simpler than clearing screen)
			fmt.Print("\033[H\033[2J")
		}

		fmt.Printf("Job: %s\n", job.Name)
		fmt.Printf("Type: %s\n", job.JobType)
		fmt.Printf("Status: %s\n", job.Status)
		if job.Site != "" {
			fmt.Printf("Site: %s\n", job.Site)
		}
		fmt.Printf("Started: %s\n", job.Start)

		if job.Status == "Success" || job.Status == "Failure" {
			if job.End != "" {
				fmt.Printf("Ended: %s\n", job.End)
			}
			if job.Duration != "" {
				fmt.Printf("Duration: %s\n", job.Duration)
			}
			return nil
		}

		if !watch {
			fmt.Println("\nJob is still running. Use --watch to follow progress.")
			return nil
		}

		time.Sleep(2 * time.Second)
	}
}
