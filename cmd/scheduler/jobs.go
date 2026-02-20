package scheduler

import (
	"fmt"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/api"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/spf13/cobra"
)

var (
	jobsSite  string
	jobsLimit int
)

var jobsCmd = &cobra.Command{
	Use:   "jobs",
	Short: "List pending background jobs",
	Long: `List pending background jobs in the queue.

Shows jobs waiting to be processed by the scheduler.

Examples:
  weg scheduler jobs
  weg scheduler jobs --limit 50
  weg scheduler jobs --site mysite.localhost`,
	RunE: runJobs,
}

func init() {
	SchedulerCmd.AddCommand(jobsCmd)
	jobsCmd.Flags().StringVar(&jobsSite, "site", "", "Site to check")
	jobsCmd.Flags().IntVar(&jobsLimit, "limit", 20, "Maximum jobs to show")
}

func runJobs(cmd *cobra.Command, args []string) error {
	benchPath, site, err := resolveSite(jobsSite)
	if err != nil {
		return err
	}

	executor := api.NewExecutor(benchPath, site, "Administrator")

	script := fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()

try:
    jobs = frappe.get_all('RQ Job',
        filters={'status': ['in', ['queued', 'started']]},
        fields=['name', 'job_name', 'queue', 'status', 'creation'],
        order_by='creation desc',
        limit=%d
    )
    print(json.dumps({"success": True, "data": jobs}, default=str))
except Exception as ex:
    print(json.dumps({"success": False, "error": str(ex)}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site, jobsLimit)

	result, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to list jobs: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("failed to list jobs: %s", result.Error)
	}

	jobs, ok := result.Data.([]any)
	if !ok {
		output.Print("No pending jobs")
		return nil
	}

	if len(jobs) == 0 {
		output.Print("No pending jobs")
		return nil
	}

	output.Printf("Pending jobs for %s:\n", site)
	output.Printf("%-40s %-15s %-10s %s", "JOB NAME", "QUEUE", "STATUS", "CREATED")
	output.Print("--------------------------------------------------------------------------------")

	for _, job := range jobs {
		j, ok := job.(map[string]any)
		if !ok {
			continue
		}
		jobName, _ := j["job_name"].(string)
		queue, _ := j["queue"].(string)
		status, _ := j["status"].(string)
		creation, _ := j["creation"].(string)

		// Truncate job name if too long
		if len(jobName) > 38 {
			jobName = jobName[:35] + "..."
		}

		output.Printf("%-40s %-15s %-10s %s", jobName, queue, status, creation)
	}

	return nil
}
