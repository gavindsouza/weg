package site

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/completion"
	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install <site-name> <app-name>",
	Short: "Install an app on a site",
	Long: `Install a Frappe app on a specific site.

The app must already be installed in the bench (use 'weg app get' first).

Examples:
  weg site install mysite.localhost erpnext`,
	Args:              cobra.ExactArgs(2),
	RunE:              runInstall,
	ValidArgsFunction: completeSiteInstallArgs,
}

// completeSiteInstallArgs provides completion for site install command.
// First arg: site name, Second arg: app name
func completeSiteInstallArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	switch len(args) {
	case 0:
		return completion.CompleteSiteNames(cmd, args, toComplete)
	case 1:
		return completion.CompleteAppNames(cmd, args, toComplete)
	default:
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

func runInstall(cmd *cobra.Command, args []string) error {
	path := "."
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	result, err := config.DetectContext(absPath)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	siteName := args[0]
	appName := args[1]

	var benchPath string
	switch result.Context {
	case config.ContextWegBench:
		benchPath = absPath
	case config.ContextWegApp:
		benchPath = filepath.Join(absPath, ".weg")
	default:
		return fmt.Errorf("not a weg-managed project")
	}

	sitesDir := filepath.Join(benchPath, "sites")
	sitePath := filepath.Join(sitesDir, siteName)
	appsDir := filepath.Join(benchPath, "apps")
	appPath := filepath.Join(appsDir, appName)

	// Check if site exists
	if _, err := os.Stat(sitePath); os.IsNotExist(err) {
		return fmt.Errorf("site %s does not exist", siteName)
	}

	// Check if app is installed
	if _, err := os.Stat(appPath); os.IsNotExist(err) {
		return fmt.Errorf("app %s is not installed. Run 'weg app get %s' first", appName, appName)
	}

	output.Infof("Installing %s on %s...", appName, siteName)

	// Run bench install-app
	benchCmd := exec.Command("bench", "--site", siteName, "install-app", appName)
	benchCmd.Dir = benchPath
	benchCmd.Stdout = os.Stdout
	benchCmd.Stderr = os.Stderr

	if err := benchCmd.Run(); err != nil {
		return fmt.Errorf("failed to install app: %w", err)
	}

	// Update state
	st, err := state.Load(absPath)
	if err != nil {
		st = state.NewState()
	}

	if siteState, ok := st.Sites[siteName]; ok {
		// Check if app already in list
		hasApp := false
		for _, a := range siteState.Apps {
			if a == appName {
				hasApp = true
				break
			}
		}
		if !hasApp {
			siteState.Apps = append(siteState.Apps, appName)
			st.Sites[siteName] = siteState
		}
	} else {
		st.AddSite(state.SiteState{
			Name: siteName,
			Apps: []string{appName},
		})
	}

	if err := st.Save(absPath); err != nil {
		output.Warningf("Failed to save state: %v", err)
	}

	output.Successf("Successfully installed %s on %s", appName, siteName)
	return nil
}
