package build

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/config"
	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var assetsCmd = &cobra.Command{
	Use:   "assets [app]",
	Short: "Build app assets",
	Long: `Build frontend assets for one or all apps.

This runs the Frappe build process which compiles JS/CSS bundles.

Examples:
  weg build assets              # Build all apps
  weg build assets myapp        # Build specific app
  weg build assets --hard       # Clean build, remove old bundles first
  weg build assets --production # Production mode with minification`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAssets,
}

var (
	assetsSite       string
	assetsHard       bool
	assetsProduction bool
)

func init() {
	BuildCmd.AddCommand(assetsCmd)
	assetsCmd.Flags().StringVarP(&assetsSite, "site", "s", "", "Site to build for")
	assetsCmd.Flags().BoolVar(&assetsHard, "hard", false, "Clean build, remove old bundles")
	assetsCmd.Flags().BoolVar(&assetsProduction, "production", false, "Production mode with minification")
}

func runAssets(cmd *cobra.Command, args []string) error {
	benchPath, site, err := resolveContext(assetsSite)
	if err != nil {
		return err
	}

	var appName string
	if len(args) > 0 {
		appName = args[0]
	}

	if assetsHard {
		fmt.Println("Cleaning old bundles...")
		cleanBundles(benchPath, site)
	}

	buildArgs := []string{"build"}
	if appName != "" {
		buildArgs = append(buildArgs, "--app", appName)
	}
	if assetsProduction {
		buildArgs = append(buildArgs, "--production")
	}

	output.Infof("Building assets for site %s...\n", site)

	// Run frappe build via bench_helper
	sitesDir := filepath.Join(benchPath, "sites")
	pythonPath := filepath.Join(benchPath, "env", "bin", "python")
	devboxArgs := []string{"run", "-c", benchPath, "--", pythonPath, "-m", "frappe.utils.bench_helper", "frappe"}
	devboxArgs = append(devboxArgs, buildArgs...)

	buildCmd := exec.Command("devbox", devboxArgs...)
	buildCmd.Dir = sitesDir
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr

	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	fmt.Println("Build completed successfully")
	return nil
}

func cleanBundles(benchPath, site string) {
	// Remove old bundle files
	assetsDir := filepath.Join(benchPath, "sites", "assets")
	if entries, err := os.ReadDir(assetsDir); err == nil {
		for _, e := range entries {
			if e.IsDir() && e.Name() != "frappe" {
				bundleDir := filepath.Join(assetsDir, e.Name(), "dist")
				os.RemoveAll(bundleDir)
			}
		}
	}
}

func resolveContext(siteName string) (string, string, error) {
	path := "."
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", "", fmt.Errorf("invalid path: %w", err)
	}

	result, err := config.DetectProjectContext(absPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to detect context: %w", err)
	}

	var benchPath string
	switch result.Context {
	case config.ContextWegBench:
		benchPath = result.BenchPath
	case config.ContextWegApp:
		benchPath = result.BenchPath
	default:
		return "", "", wegerrors.NotInProject(absPath)
	}

	site := siteName
	if site == "" {
		st, err := state.Load(absPath)
		if err == nil {
			site = st.GetDefaultSite()
		}
		if site == "" {
			currentSitePath := filepath.Join(benchPath, "sites", "currentsite.txt")
			data, _ := os.ReadFile(currentSitePath)
			site = strings.TrimSpace(string(data))
		}
	}

	if site == "" {
		return "", "", fmt.Errorf("no site specified and no default site found")
	}

	return benchPath, site, nil
}
