package docker

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop Docker containers",
	Long: `Stop and remove Docker Compose containers.

Examples:
  weg docker down           # Stop containers
  weg docker down -v        # Stop and remove volumes`,
	RunE: runDown,
}

var downVolumes bool

func init() {
	downCmd.Flags().BoolVarP(&downVolumes, "volumes", "v", false, "Remove volumes")
}

func runDown(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	cmdArgs := []string{"compose", "down"}
	if downVolumes {
		cmdArgs = append(cmdArgs, "-v")
	}

	fmt.Println("Stopping containers...")

	execCmd := exec.Command("docker", cmdArgs...)
	execCmd.Dir = cwd
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	if err := execCmd.Run(); err != nil {
		return fmt.Errorf("failed to stop containers: %w", err)
	}

	fmt.Println("Containers stopped")
	return nil
}
