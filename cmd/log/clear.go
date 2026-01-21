package log

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/prompt"
	"github.com/spf13/cobra"
)

var clearCmd = &cobra.Command{
	Use:   "clear [type]",
	Short: "Clear log files",
	Long: `Clear log files to free up disk space.

Log types:
  web      - Web server logs
  worker   - Background worker logs
  schedule - Scheduler logs
  all      - All logs (default)

Examples:
  weg log clear           # Clear all logs
  weg log clear worker    # Clear only worker logs`,
	Args: cobra.MaximumNArgs(1),
	RunE: runClear,
}

var (
	clearSite string
	clearAll  bool
)

func init() {
	LogCmd.AddCommand(clearCmd)
	clearCmd.Flags().StringVarP(&clearSite, "site", "s", "", "Site to clear logs for")
	clearCmd.Flags().BoolVar(&clearAll, "yes", false, "Skip confirmation")
}

func runClear(cmd *cobra.Command, args []string) error {
	benchPath, site, err := resolveContext(clearSite)
	if err != nil {
		return err
	}

	logType := "all"
	if len(args) > 0 {
		logType = args[0]
	}

	logFiles := findLogFiles(benchPath, site, logType)
	if len(logFiles) == 0 {
		output.Print("No log files found")
		return nil
	}

	if !clearAll {
		output.Printf("This will clear %d log file(s):", len(logFiles))
		for _, f := range logFiles {
			info, err := os.Stat(f)
			size := ""
			if err == nil {
				size = formatSize(info.Size())
			}
			output.Printf("  - %s (%s)", filepath.Base(f), size)
		}
		output.Print("")
		if !prompt.Confirm("Continue?") {
			output.Print("Cancelled")
			return nil
		}
	}

	cleared := 0
	var totalSize int64

	for _, f := range logFiles {
		info, err := os.Stat(f)
		if err == nil {
			totalSize += info.Size()
		}

		// Truncate the file instead of deleting (keeps the file but empties it)
		if err := os.Truncate(f, 0); err != nil {
			output.Warningf("Failed to clear %s: %v", filepath.Base(f), err)
			continue
		}
		cleared++
	}

	output.Successf("Cleared %d log file(s), freed %s", cleared, formatSize(totalSize))
	return nil
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
