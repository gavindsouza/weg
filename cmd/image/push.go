package image

import (
	"os"
	"os/exec"

	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:   "push <image-tag>",
	Short: "Push an image to a registry",
	Long: `Push a container image to a registry.

Examples:
  weg image push myapp:latest
  weg image push ghcr.io/user/myapp:v1.0`,
	Args: cobra.ExactArgs(1),
	RunE: runPush,
}

func runPush(cmd *cobra.Command, args []string) error {
	imageTag := args[0]

	// Determine container runtime
	runtime := "docker"
	if _, err := exec.LookPath("podman"); err == nil {
		runtime = "podman"
	}

	output.Infof("Pushing %s...\n", imageTag)

	execCmd := exec.Command(runtime, "push", imageTag)
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	if err := execCmd.Run(); err != nil {
		return wegerrors.Operation("push", "", err)
	}

	output.Successf("Successfully pushed %s\n", imageTag)
	return nil
}
