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
		output.Printf("Account: %s (%s)", user.Name, user.Email)
	} else {
		output.Printf("Account: %s", user.Email)
	}
	if user.Team != "" {
		output.Printf("Team: %s", user.Team)
	}
	output.Print("")

	// Get benches
	benches, err := client.ListBenches("")
	if err == nil && len(benches) > 0 {
		output.Printf("Benches (%d):", len(benches))
		t := output.NewTable("Name", "Version", "Sites", "Apps", "Status")
		for _, b := range benches {
			t.Row(b.Name, b.FrappeVersion, b.SiteCount, b.AppCount, b.Status)
		}
		t.Flush()
		output.Print("")
	}

	// Get sites
	sites, err := client.ListSites("")
	if err == nil && len(sites) > 0 {
		output.Printf("Sites (%d):", len(sites))
		t := output.NewTable("Name", "Status", "Plan", "Region")
		for _, s := range sites {
			t.Row(s.Name, s.Status, s.Plan, s.Region)
		}
		t.Flush()
		output.Print("")
	}

	if (benches == nil || len(benches) == 0) && (sites == nil || len(sites) == 0) {
		output.Print("No benches or sites found.")
		output.Print("\nYou may only have marketplace apps. Use 'weg cloud mp' to check.")
	}

	return nil
}

func showSiteStatus(client *cloud.Client, siteName string) error {
	site, err := client.GetSiteDetail(siteName)
	if err != nil {
		return fmt.Errorf("failed to get site: %w", err)
	}

	output.Printf("Site: %s", site.Name)
	output.Printf("Status: %s", site.Status)
	if site.BenchTitle != "" {
		output.Printf("Bench: %s", site.BenchTitle)
	}
	if site.FrappeVersion != "" {
		output.Printf("Frappe: %s", site.FrappeVersion)
	}
	output.Printf("Created: %s", site.CreatedAt)

	if site.UpdateAvailable {
		output.Print("\n[!] Updates available")
	}

	// Show installed apps
	if len(site.InstalledApps) > 0 {
		output.Print("\nInstalled Apps:")
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
		output.Print("\nRunning Jobs:")
		for _, job := range runningJobs {
			output.Printf("  - %s (%s) started %s", job.JobType, job.Name, job.Start)
		}
	}

	// Show recent jobs
	recentJobs, err := client.GetSiteJobs(siteName, 5)
	if err == nil && len(recentJobs) > 0 {
		output.Print("\nRecent Jobs:")
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

	output.Printf("Bench: %s", benchName)
	output.Print("\nRecent Jobs:")

	if len(jobs) == 0 {
		output.Print("  No recent jobs")
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

		output.Printf("Job: %s", job.Name)
		output.Printf("Type: %s", job.JobType)
		output.Printf("Status: %s", job.Status)
		if job.Site != "" {
			output.Printf("Site: %s", job.Site)
		}
		output.Printf("Started: %s", job.Start)

		if job.Status == "Success" || job.Status == "Failure" {
			if job.End != "" {
				output.Printf("Ended: %s", job.End)
			}
			if job.Duration != "" {
				output.Printf("Duration: %s", job.Duration)
			}
			return nil
		}

		if !watch {
			output.Print("\nJob is still running. Use --watch to follow progress.")
			return nil
		}

		time.Sleep(2 * time.Second)
	}
}
