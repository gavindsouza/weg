package docker

import (
	"fmt"
	"os"
	"os/exec"

	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/spf13/cobra"
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start Docker containers",
	Long: `Start Docker Compose containers for the project.

Examples:
  weg docker up           # Start in foreground
  weg docker up -d        # Start in background (detached)
  weg docker up --build   # Rebuild images before starting`,
	RunE: runUp,
}

var (
	upDetached bool
	upBuild    bool
)

func init() {
	upCmd.Flags().BoolVarP(&upDetached, "detach", "d", false, "Run in background")
	upCmd.Flags().BoolVar(&upBuild, "build", false, "Build images before starting")
}

func runUp(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Check if docker-compose.yml exists
	if _, err := os.Stat("docker-compose.yml"); os.IsNotExist(err) {
		return wegerrors.NotFound("docker-compose.yml", "")
	}

	cmdArgs := []string{"compose", "up"}
	if upDetached {
		cmdArgs = append(cmdArgs, "-d")
	}
	if upBuild {
		cmdArgs = append(cmdArgs, "--build")
	}

	output.Print("Starting containers...")

	execCmd := exec.Command("docker", cmdArgs...)
	execCmd.Dir = cwd
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	execCmd.Stdin = os.Stdin

	if err := execCmd.Run(); err != nil {
		return fmt.Errorf("failed to start containers: %w", err)
	}

	if upDetached {
		output.Print("")
		output.Print("Containers started in background")
		output.Print("  weg docker ps      # View status")
		output.Print("  weg docker logs    # View logs")
		output.Print("  weg docker down    # Stop containers")
	}

	return nil
}
