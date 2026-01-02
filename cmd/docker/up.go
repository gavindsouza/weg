package docker

import (
	"fmt"
	"os"
	"os/exec"

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
		return fmt.Errorf("docker-compose.yml not found. Run 'weg docker init' first")
	}

	cmdArgs := []string{"compose", "up"}
	if upDetached {
		cmdArgs = append(cmdArgs, "-d")
	}
	if upBuild {
		cmdArgs = append(cmdArgs, "--build")
	}

	fmt.Println("Starting containers...")

	execCmd := exec.Command("docker", cmdArgs...)
	execCmd.Dir = cwd
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	execCmd.Stdin = os.Stdin

	if err := execCmd.Run(); err != nil {
		return fmt.Errorf("failed to start containers: %w", err)
	}

	if upDetached {
		fmt.Println()
		fmt.Println("✓ Containers started in background")
		fmt.Println("  weg docker ps      # View status")
		fmt.Println("  weg docker logs    # View logs")
		fmt.Println("  weg docker down    # Stop containers")
	}

	return nil
}
