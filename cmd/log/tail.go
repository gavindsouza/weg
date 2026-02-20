package log

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/gavindsouza/weg/internal/config"
	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var tailCmd = &cobra.Command{
	Use:   "tail [type]",
	Short: "Tail log files",
	Long: `Tail Frappe log files in real-time.

Log types:
  web      - Web server logs (gunicorn/werkzeug)
  worker   - Background worker logs
  schedule - Scheduler logs
  error    - Error logs only
  all      - All logs (default)

Examples:
  weg log tail              # Tail all logs
  weg log tail web          # Tail web server logs
  weg log tail worker       # Tail worker logs
  weg log tail --lines 100  # Show last 100 lines first`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTail,
}

var (
	tailSite  string
	tailLines int
)

func init() {
	LogCmd.AddCommand(tailCmd)
	tailCmd.Flags().StringVarP(&tailSite, "site", "s", "", "Site to view logs for")
	tailCmd.Flags().IntVarP(&tailLines, "lines", "n", 20, "Number of lines to show initially")
}

func runTail(cmd *cobra.Command, args []string) error {
	benchPath, site, err := resolveContext(tailSite)
	if err != nil {
		return err
	}

	logType := "all"
	if len(args) > 0 {
		logType = args[0]
	}

	// Find log files based on type
	logFiles := findLogFiles(benchPath, site, logType)
	if len(logFiles) == 0 {
		return fmt.Errorf("no log files found for type '%s'", logType)
	}

	output.Infof("Tailing logs for %s (type: %s)...\n", site, logType)
	fmt.Println("Press Ctrl+C to stop")

	// Use tail -f to follow logs
	tailArgs := []string{"-f", "-n", fmt.Sprintf("%d", tailLines)}
	tailArgs = append(tailArgs, logFiles...)

	tailProcess := exec.Command("tail", tailArgs...)
	tailProcess.Stdout = os.Stdout
	tailProcess.Stderr = os.Stderr

	// Handle interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nStopping log tail...")
		if tailProcess.Process != nil {
			tailProcess.Process.Signal(syscall.SIGTERM)
		}
	}()

	if err := tailProcess.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == -1 {
				return nil // Interrupted
			}
		}
		return fmt.Errorf("tail failed: %w", err)
	}

	return nil
}

func findLogFiles(benchPath, site, logType string) []string {
	var patterns []string

	siteLogs := filepath.Join(benchPath, "sites", site, "logs")
	benchLogs := filepath.Join(benchPath, "logs")

	switch logType {
	case "web":
		patterns = []string{
			filepath.Join(siteLogs, "*.log"),
			filepath.Join(benchLogs, "web.*.log"),
		}
	case "worker":
		patterns = []string{
			filepath.Join(benchLogs, "worker.*.log"),
			filepath.Join(benchLogs, "frappe.log"),
		}
	case "schedule", "scheduler":
		patterns = []string{
			filepath.Join(benchLogs, "schedule.*.log"),
			filepath.Join(benchLogs, "scheduler.log"),
		}
	case "error":
		patterns = []string{
			filepath.Join(siteLogs, "*error*.log"),
			filepath.Join(benchLogs, "*error*.log"),
		}
	case "all":
		patterns = []string{
			filepath.Join(siteLogs, "*.log"),
			filepath.Join(benchLogs, "*.log"),
		}
	default:
		// Try to find matching log file
		patterns = []string{
			filepath.Join(siteLogs, logType+".log"),
			filepath.Join(siteLogs, "*"+logType+"*.log"),
			filepath.Join(benchLogs, logType+".log"),
			filepath.Join(benchLogs, "*"+logType+"*.log"),
		}
	}

	var logFiles []string
	seen := make(map[string]bool)

	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, m := range matches {
			if !seen[m] {
				seen[m] = true
				logFiles = append(logFiles, m)
			}
		}
	}

	return logFiles
}

func resolveContext(siteName string) (string, string, error) {
	path := "."
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", "", fmt.Errorf("invalid path: %w", err)
	}

	result, err := config.DetectProjectContext(absPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to detect context: %w", err)
	}

	var benchPath string
	switch result.Context {
	case config.ContextWegBench:
		benchPath = result.BenchPath
	case config.ContextWegApp:
		benchPath = result.BenchPath
	default:
		return "", "", wegerrors.NotInProject(absPath)
	}

	site := siteName
	if site == "" {
		st, err := state.Load(absPath)
		if err == nil {
			site = st.GetDefaultSite()
		}
		if site == "" {
			currentSitePath := filepath.Join(benchPath, "sites", "currentsite.txt")
			data, _ := os.ReadFile(currentSitePath)
			site = strings.TrimSpace(string(data))
		}
	}

	if site == "" {
		return "", "", fmt.Errorf("no site specified and no default site found")
	}

	return benchPath, site, nil
}
