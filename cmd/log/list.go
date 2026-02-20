package log

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gavindsouza/weg/internal/output"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available log files",
	Long: `List all available log files and their sizes.

Examples:
  weg log list`,
	RunE: runList,
}

var listSite string

func init() {
	LogCmd.AddCommand(listCmd)
	listCmd.Flags().StringVarP(&listSite, "site", "s", "", "Site to list logs for")
}

func runList(cmd *cobra.Command, args []string) error {
	benchPath, site, err := resolveContext(listSite)
	if err != nil {
		return err
	}

	siteLogs := filepath.Join(benchPath, "sites", site, "logs")
	benchLogs := filepath.Join(benchPath, "logs")

	output.Printf("Log files for %s:\n", site)

	// Site-specific logs
	if entries, err := os.ReadDir(siteLogs); err == nil && len(entries) > 0 {
		output.Printf("Site logs (%s):", siteLogs)
		output.Printf("%-40s %10s %s", "FILE", "SIZE", "MODIFIED")
		output.Print(strings.Repeat("-", 70))
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".log") {
				info, err := e.Info()
				if err != nil {
					continue
				}
				output.Printf("%-40s %10s %s",
					e.Name(),
					formatSize(info.Size()),
					info.ModTime().Format(time.RFC822))
			}
		}
		output.Print("")
	}

	// Bench logs
	if entries, err := os.ReadDir(benchLogs); err == nil && len(entries) > 0 {
		output.Printf("Bench logs (%s):", benchLogs)
		output.Printf("%-40s %10s %s", "FILE", "SIZE", "MODIFIED")
		output.Print(strings.Repeat("-", 70))
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".log") {
				info, err := e.Info()
				if err != nil {
					continue
				}
				output.Printf("%-40s %10s %s",
					e.Name(),
					formatSize(info.Size()),
					info.ModTime().Format(time.RFC822))
			}
		}
	}

	return nil
}
