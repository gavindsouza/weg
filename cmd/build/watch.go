package build

import (
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/output"
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

	output.Infof("Starting asset watcher for site %s...", site)
	output.Print("Press Ctrl+C to stop")

	// Run frappe watch via bench_helper
	sitesDir := filepath.Join(benchPath, "sites")
	pythonPath := filepath.Join(benchPath, "env", "bin", "python")
	devboxArgs := []string{"run", "-c", benchPath, "--", pythonPath, "-m", "frappe.utils.bench_helper", "frappe"}
	devboxArgs = append(devboxArgs, watchArgs...)

	watchCmd := exec.Command("devbox", devboxArgs...)
	watchCmd.Dir = sitesDir
	watchCmd.Stdout = os.Stdout
	watchCmd.Stderr = os.Stderr

	// Handle interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		output.Print("\nStopping watcher...")
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
		return wegerrors.Operation("watch", "", err)
	}

	return nil
}
