package log

import (
	"fmt"
	"os"
	"path/filepath"

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
		fmt.Println("No log files found")
		return nil
	}

	if !clearAll {
		fmt.Printf("This will clear %d log file(s):\n", len(logFiles))
		for _, f := range logFiles {
			info, err := os.Stat(f)
			size := ""
			if err == nil {
				size = formatSize(info.Size())
			}
			fmt.Printf("  - %s (%s)\n", filepath.Base(f), size)
		}
		fmt.Print("\nContinue? [y/N]: ")
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Cancelled")
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
			fmt.Printf("Failed to clear %s: %v\n", filepath.Base(f), err)
			continue
		}
		cleared++
	}

	fmt.Printf("Cleared %d log file(s), freed %s\n", cleared, formatSize(totalSize))
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
