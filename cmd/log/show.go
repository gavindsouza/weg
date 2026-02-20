package log

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show [type]",
	Short: "Show recent log entries",
	Long: `Show recent log entries without following.

Examples:
  weg log show              # Show recent logs
  weg log show web          # Show web server logs
  weg log show -n 100       # Show last 100 lines`,
	Args: cobra.MaximumNArgs(1),
	RunE: runShow,
}

var (
	showSite  string
	showLines int
)

func init() {
	LogCmd.AddCommand(showCmd)
	showCmd.Flags().StringVarP(&showSite, "site", "s", "", "Site to view logs for")
	showCmd.Flags().IntVarP(&showLines, "lines", "n", 50, "Number of lines to show")
}

func runShow(cmd *cobra.Command, args []string) error {
	benchPath, site, err := resolveContext(showSite)
	if err != nil {
		return err
	}

	logType := "all"
	if len(args) > 0 {
		logType = args[0]
	}

	logFiles := findLogFiles(benchPath, site, logType)
	if len(logFiles) == 0 {
		return wegerrors.NotFound("log type", logType)
	}

	// Read and combine log entries from all files
	entries := []logEntry{}
	for _, logFile := range logFiles {
		fileEntries := readLogFile(logFile, showLines)
		entries = append(entries, fileEntries...)
	}

	// Sort by timestamp if available
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].timestamp.Before(entries[j].timestamp)
	})

	// Show only the last N entries
	if len(entries) > showLines {
		entries = entries[len(entries)-showLines:]
	}

	for _, e := range entries {
		output.Print(e.line)
	}

	return nil
}

type logEntry struct {
	line      string
	timestamp time.Time
	source    string
}

func readLogFile(path string, maxLines int) []logEntry {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	source := filepath.Base(path)

	// Read from end of file
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	// Take last maxLines
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}

	entries := make([]logEntry, 0, len(lines))
	for _, line := range lines {
		ts := parseTimestamp(line)
		entries = append(entries, logEntry{
			line:      fmt.Sprintf("[%s] %s", source, line),
			timestamp: ts,
			source:    source,
		})
	}

	return entries
}

func parseTimestamp(line string) time.Time {
	// Try common timestamp formats
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"[2006-01-02 15:04:05]",
		time.RFC3339,
	}

	for _, format := range formats {
		if len(line) >= len(format) {
			// Try to extract timestamp from beginning of line
			sample := line
			if len(sample) > 30 {
				sample = sample[:30]
			}
			if t, err := time.Parse(format, strings.TrimSpace(sample[:len(format)])); err == nil {
				return t
			}
		}
	}

	return time.Now()
}
