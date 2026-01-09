package cloud

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy [site]",
	Short: "Deploy to Frappe Cloud",
	Long: `Deploy the current app or bench to a Frappe Cloud site.

Examples:
  weg cloud deploy                    # Deploy to default site
  weg cloud deploy mysite.frappe.cloud # Deploy to specific site
  weg cloud deploy --bench mybench    # Deploy to a bench
  weg cloud deploy --create           # Create site if not exists`,
	RunE: runDeploy,
}

var (
	deployBench  string
	deployCreate bool
	deployDryRun bool
)

func init() {
	deployCmd.Flags().StringVar(&deployBench, "bench", "", "Target bench name")
	deployCmd.Flags().BoolVar(&deployCreate, "create", false, "Create site/bench if not exists")
	deployCmd.Flags().BoolVar(&deployDryRun, "dry-run", false, "Preview deployment without applying")
}

func runDeploy(cmd *cobra.Command, args []string) error {
	client, err := getAuthenticatedClient("")
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	result, err := config.DetectContext(cwd)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	if result.Context != config.ContextWegApp && result.Context != config.ContextWegBench {
		return fmt.Errorf("not in a weg-managed project")
	}

	appName := result.AppName
	if appName == "" {
		appName = filepath.Base(cwd)
	}

	// Determine target site
	var siteName string
	if len(args) > 0 {
		siteName = args[0]
	} else {
		// Try to get from config or prompt
		fmt.Println("No site specified. Available sites:")
		sites, err := client.ListSites("")
		if err != nil {
			return fmt.Errorf("failed to list sites: %w", err)
		}
		for i, site := range sites {
			fmt.Printf("  [%d] %s\n", i+1, site.Name)
		}
		return fmt.Errorf("please specify a site: weg cloud deploy <site>")
	}

	fmt.Printf("Deploying %s to %s...\n", appName, siteName)

	if deployDryRun {
		fmt.Println("\nDry run - no changes applied")
		return nil
	}

	// Trigger deployment
	deploy, err := client.DeployToSite(siteName, appName)
	if err != nil {
		return fmt.Errorf("deployment failed: %w", err)
	}

	fmt.Printf("\nDeployment started: %s\n", deploy.ID)
	fmt.Printf("Track progress: weg cloud status %s\n", deploy.ID)

	return nil
}
