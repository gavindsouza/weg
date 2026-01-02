package docker

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var psCmd = &cobra.Command{
	Use:   "ps",
	Short: "List containers",
	Long: `List running Docker Compose containers.

Examples:
  weg docker ps
  weg docker ps -a    # Include stopped containers`,
	RunE: runPs,
}

var psAll bool

func init() {
	psCmd.Flags().BoolVarP(&psAll, "all", "a", false, "Include stopped containers")
}

func runPs(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	cmdArgs := []string{"compose", "ps"}
	if psAll {
		cmdArgs = append(cmdArgs, "-a")
	}

	execCmd := exec.Command("docker", cmdArgs...)
	execCmd.Dir = cwd
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	return execCmd.Run()
}
