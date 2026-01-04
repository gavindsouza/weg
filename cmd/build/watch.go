package build

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

var watchCmd = &cobra.Command{
	Use:   "watch [app]",
	Short: "Watch and rebuild assets on change",
	Long: `Start the asset watcher for development.

Watches for file changes and automatically rebuilds assets.
Press Ctrl+C to stop.

Examples:
  weg build watch              # Watch all apps
  weg build watch myapp        # Watch specific app`,
	Args: cobra.MaximumNArgs(1),
	RunE: runWatch,
}

var watchSite string

func init() {
	BuildCmd.AddCommand(watchCmd)
	watchCmd.Flags().StringVarP(&watchSite, "site", "s", "", "Site to watch for")
}

func runWatch(cmd *cobra.Command, args []string) error {
	benchPath, site, err := resolveContext(watchSite)
	if err != nil {
		return err
	}

	var appName string
	if len(args) > 0 {
		appName = args[0]
	}

	watchArgs := []string{"watch"}
	if appName != "" {
		watchArgs = append(watchArgs, "--apps", appName)
	}

	fmt.Printf("Starting asset watcher for site %s...\n", site)
	fmt.Println("Press Ctrl+C to stop")

	// Run frappe watch via bench_helper
	sitesDir := filepath.Join(benchPath, "sites")
	shellCmd := fmt.Sprintf("cd %s && ../.venv/bin/python -m frappe.utils.bench_helper frappe %s",
		sitesDir, strings.Join(watchArgs, " "))

	watchCmd := exec.Command("devbox", "run", "-c", benchPath, "--", "sh", "-c", shellCmd)
	watchCmd.Stdout = os.Stdout
	watchCmd.Stderr = os.Stderr

	// Handle interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nStopping watcher...")
		if watchCmd.Process != nil {
			watchCmd.Process.Signal(syscall.SIGTERM)
		}
	}()

	if err := watchCmd.Run(); err != nil {
		// Ignore interrupt errors
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == -1 {
				return nil
			}
		}
		return fmt.Errorf("watch failed: %w", err)
	}

	return nil
}
