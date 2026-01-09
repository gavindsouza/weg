package cloud

import (
	"fmt"

	"github.com/spf13/cobra"
)

var cloudLogsCmd = &cobra.Command{
	Use:   "logs [site]",
	Short: "View site logs from Frappe Cloud",
	Long: `Stream logs from a Frappe Cloud site.

Examples:
  weg cloud logs mysite.frappe.cloud           # All logs
  weg cloud logs mysite --type web             # Web server logs
  weg cloud logs mysite --type worker          # Background worker logs
  weg cloud logs mysite --type scheduler       # Scheduler logs
  weg cloud logs mysite --follow               # Stream logs`,
	RunE: runCloudLogs,
}

var (
	cloudLogsType   string
	cloudLogsFollow bool
	cloudLogsTail   int
)

func init() {
	cloudLogsCmd.Flags().StringVar(&cloudLogsType, "type", "", "Log type (web, worker, scheduler)")
	cloudLogsCmd.Flags().BoolVarP(&cloudLogsFollow, "follow", "f", false, "Follow log output")
	cloudLogsCmd.Flags().IntVarP(&cloudLogsTail, "tail", "n", 100, "Number of lines to show")
}

func runCloudLogs(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("site name is required")
	}

	client, err := getAuthenticatedClient("")
	if err != nil {
		return err
	}

	siteName := args[0]

	logs, err := client.GetSiteLogs(siteName, cloudLogsType, cloudLogsTail)
	if err != nil {
		return fmt.Errorf("failed to get logs: %w", err)
	}

	for _, line := range logs {
		fmt.Println(line)
	}

	if cloudLogsFollow {
		// Stream logs
		logChan, errChan := client.StreamSiteLogs(siteName, cloudLogsType)
		for {
			select {
			case line := <-logChan:
				fmt.Println(line)
			case err := <-errChan:
				if err != nil {
					return fmt.Errorf("log stream error: %w", err)
				}
				return nil
			}
		}
	}

	return nil
}
