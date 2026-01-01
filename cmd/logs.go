package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/services"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs [service]",
	Short: "View service logs",
	Long: `View logs from running services.

Without arguments, shows logs from all services. Specify a service name
to view logs from that service only.

Examples:
  weg logs              # Show all logs
  weg logs web          # Show web server logs
  weg logs -f           # Follow logs in real-time
  weg logs -f worker    # Follow worker logs`,
	RunE: runLogs,
}

var follow bool

func init() {
	rootCmd.AddCommand(logsCmd)
	logsCmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")
}

func runLogs(cmd *cobra.Command, args []string) error {
	path := "."
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Detect context
	result, err := config.DetectContext(absPath)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	// Determine bench path
	var benchPath string
	switch result.Context {
	case config.ContextWegApp:
		benchPath = filepath.Join(absPath, ".weg")
	case config.ContextWegBench:
		benchPath = absPath
	default:
		return fmt.Errorf("not a weg-managed project")
	}

	service := ""
	if len(args) > 0 {
		service = args[0]
	}

	mgr := services.NewManager(benchPath)
	return mgr.Logs(service, follow)
}
