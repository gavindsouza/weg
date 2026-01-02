package image

import (
	"github.com/spf13/cobra"
)

var ImageCmd = &cobra.Command{
	Use:   "image",
	Short: "Build and manage container images",
	Long: `Build and manage OCI container images for Frappe apps.

Commands for building production-ready container images that can be
deployed to any container runtime (Docker, Podman, Kubernetes).

Examples:
  weg image build              # Build image for current project
  weg image build --push       # Build and push to registry
  weg image list               # List local images
  weg image push <tag>         # Push image to registry`,
}

func init() {
	ImageCmd.AddCommand(buildCmd)
	ImageCmd.AddCommand(listCmd)
	ImageCmd.AddCommand(pushCmd)
}
