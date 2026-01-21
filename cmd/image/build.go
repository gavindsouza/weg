package image

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/container"
	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build a container image",
	Long: `Build a production container image for the current project.

Creates a multi-stage Docker image optimized for production:
- Stage 1: Builder - installs apps and builds assets
- Stage 2: Production - minimal runtime image
- Stage 3-6: Specialized targets (web, worker, scheduler, socketio)

Examples:
  weg image build                           # Build with defaults
  weg image build --tag myapp:v1.0          # Custom tag
  weg image build --target web              # Build only web target
  weg image build --push --registry ghcr.io # Build and push
  weg image build --platform linux/arm64    # Build for ARM`,
	RunE: runBuild,
}

var (
	buildTag        string
	buildRegistry   string
	buildTarget     string
	buildPlatform   string
	buildPush       bool
	buildNoCache    bool
	buildFrappeBase string
)

func init() {
	buildCmd.Flags().StringVarP(&buildTag, "tag", "t", "", "Image tag (default: <app-name>:latest)")
	buildCmd.Flags().StringVar(&buildRegistry, "registry", "", "Registry to tag for (e.g., ghcr.io/user)")
	buildCmd.Flags().StringVar(&buildTarget, "target", "production", "Build target (web, worker, scheduler, socketio, production, all)")
	buildCmd.Flags().StringVar(&buildPlatform, "platform", "linux/amd64", "Target platform(s)")
	buildCmd.Flags().BoolVar(&buildPush, "push", false, "Push image after building")
	buildCmd.Flags().BoolVar(&buildNoCache, "no-cache", false, "Build without using cache")
	buildCmd.Flags().StringVar(&buildFrappeBase, "base", "frappe/bench:latest", "Base image to use")
}

func runBuild(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	result, err := config.DetectContext(cwd)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	var benchPath string
	var appName string

	switch result.Context {
	case config.ContextWegBench:
		benchPath = cwd
		appName = filepath.Base(cwd)
	case config.ContextWegApp:
		benchPath = filepath.Join(cwd, ".weg")
		appName = filepath.Base(cwd)
	default:
		return fmt.Errorf("not in a weg-managed project")
	}

	// Get list of apps
	appsDir := filepath.Join(benchPath, "apps")
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		return fmt.Errorf("failed to read apps directory: %w", err)
	}

	var apps []string
	for _, entry := range entries {
		if entry.IsDir() {
			apps = append(apps, entry.Name())
		}
	}

	if len(apps) == 0 {
		return fmt.Errorf("no apps found in %s", appsDir)
	}

	// Build options
	opts := container.ImageOptions{
		BenchPath:  benchPath,
		AppName:    appName,
		Apps:       apps,
		Tag:        buildTag,
		Registry:   buildRegistry,
		Target:     buildTarget,
		Platform:   buildPlatform,
		FrappeBase: buildFrappeBase,
		Push:       buildPush,
		NoCache:    buildNoCache,
		Verbose:    true,
	}

	fmt.Println("Building container image...")
	fmt.Printf("  Apps: %v\n", apps)
	fmt.Printf("  Target: %s\n", buildTarget)
	fmt.Printf("  Platform: %s\n", buildPlatform)
	fmt.Println()

	if err := container.BuildImage(opts); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	tag := buildTag
	if tag == "" {
		tag = fmt.Sprintf("%s:latest", appName)
	}
	if buildRegistry != "" {
		tag = fmt.Sprintf("%s/%s", buildRegistry, tag)
	}

	fmt.Println()
	fmt.Printf("Image built: %s\n", tag)

	if buildPush {
		fmt.Println("Image pushed to registry")
	}

	return nil
}
