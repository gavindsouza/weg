package docker

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs [service]",
	Short: "View container logs",
	Long: `View logs from Docker Compose containers.

Examples:
  weg docker logs           # All services
  weg docker logs web       # Specific service
  weg docker logs -f        # Follow logs`,
	RunE: runLogs,
}

var logsFollow bool

func init() {
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Follow log output")
}

func runLogs(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	cmdArgs := []string{"compose", "logs"}
	if logsFollow {
		cmdArgs = append(cmdArgs, "-f")
	}
	cmdArgs = append(cmdArgs, args...)

	execCmd := exec.Command("docker", cmdArgs...)
	execCmd.Dir = cwd
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	return execCmd.Run()
}
