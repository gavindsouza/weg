package image

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/config"
	wegoutput "github.com/gavindsouza/weg/internal/output"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List container images",
	Long: `List container images for the current project.

Shows all local images that match the current app name.

Examples:
  weg image list
  weg image list --all`,
	RunE: runList,
}

var listAll bool

func init() {
	listCmd.Flags().BoolVar(&listAll, "all", false, "Show all images, not just project images")
}

func runList(cmd *cobra.Command, args []string) error {
	// Determine container runtime
	runtime := "docker"
	if _, err := exec.LookPath("podman"); err == nil {
		runtime = "podman"
	}

	var filter string
	if !listAll {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		result, _ := config.DetectProjectContext(cwd)
		if result.Context == config.ContextWegBench || result.Context == config.ContextWegApp {
			appName := filepath.Base(cwd)
			filter = appName
		}
	}

	// Build command
	cmdArgs := []string{"images", "--format", "table {{.Repository}}\t{{.Tag}}\t{{.Size}}\t{{.CreatedSince}}"}
	if filter != "" {
		cmdArgs = append(cmdArgs, "--filter", fmt.Sprintf("reference=*%s*", filter))
	}

	execCmd := exec.Command(runtime, cmdArgs...)
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	if err := execCmd.Run(); err != nil {
		// If no images found, show friendly message
		if filter != "" {
			wegoutput.Printf("No images found for '%s'", filter)
			wegoutput.Print("\nBuild an image with: weg image build")
			return nil
		}
		return err
	}

	return nil
}

// Helper function to get image details
func getImageDetails(imageName string) (string, error) {
	runtime := "docker"
	if _, err := exec.LookPath("podman"); err == nil {
		runtime = "podman"
	}

	cmd := exec.Command(runtime, "inspect", imageName, "--format", "{{.Size}}")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}
