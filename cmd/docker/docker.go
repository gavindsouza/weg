package docker

import (
	"github.com/spf13/cobra"
)

var DockerCmd = &cobra.Command{
	Use:   "docker",
	Short: "Docker Compose operations",
	Long: `Manage Docker Compose deployments for Frappe projects.

Generate docker-compose.yml and manage container lifecycle
for local development or production deployment.

Examples:
  weg docker init              # Generate docker-compose.yml
  weg docker up                # Start containers
  weg docker down              # Stop containers
  weg docker logs              # View logs
  weg docker ps                # List containers`,
}

func init() {
	DockerCmd.AddCommand(initCmd)
	DockerCmd.AddCommand(upCmd)
	DockerCmd.AddCommand(downCmd)
	DockerCmd.AddCommand(logsCmd)
	DockerCmd.AddCommand(psCmd)
}
