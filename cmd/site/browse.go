package site

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/completion"
	"github.com/gavindsouza/weg/internal/config"
	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/runtime"
	"github.com/spf13/cobra"
)

var browseCmd = &cobra.Command{
	Use:   "browse [site]",
	Short: "Open site in browser with auto-login",
	Long: `Opens the site in browser, automatically logging in as Administrator.

Examples:
  weg site browse                    # Open default site as Administrator
  weg site browse --user hr@test.com # Open as specific user
  weg site browse mysite.localhost   # Open specific site`,
	RunE:              runBrowse,
	SilenceUsage:      true,
	ValidArgsFunction: completion.CompleteSiteNamesForArg(0),
}

var browseUser string

func init() {
	SiteCmd.AddCommand(browseCmd)
	browseCmd.Flags().StringVarP(&browseUser, "user", "u", "Administrator", "User to login as")
}

func runBrowse(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	result, err := config.DetectContext(cwd)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	var benchPath string
	switch result.Context {
	case config.ContextWegBench:
		benchPath = cwd
	case config.ContextWegApp:
		benchPath = filepath.Join(cwd, ".weg")
	default:
		return wegerrors.NotInProject(cwd)
	}

	// Get site
	var site string
	if len(args) > 0 {
		site = args[0]
	} else {
		site = getDefaultSiteForBrowse(benchPath)
		if site == "" {
			return fmt.Errorf("no site found. Create one with 'weg sync'")
		}
	}

	// Check for runtime config to get correct port
	rtConfig, err := runtime.Load(benchPath)
	if err != nil {
		// Fall back to default port if runtime config doesn't exist
		rtConfig = &runtime.Config{
			Ports: runtime.DefaultPorts(),
		}
	}

	// Run frappe browse via devbox
	sitesDir := filepath.Join(benchPath, "sites")
	shellCmd := fmt.Sprintf("cd %s && ../env/bin/python -m frappe.utils.bench_helper frappe --site %s browse --user %s",
		sitesDir, site, browseUser)

	execCmd := exec.Command("devbox", "run", "-c", benchPath, "--", "sh", "-c", shellCmd)
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	execCmd.Stdin = os.Stdin

	if err := execCmd.Run(); err != nil {
		// Check if it's just stderr output (not a real error)
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 0 {
			return fmt.Errorf("browse failed: %w", err)
		}
	}

	// Also print the URL for convenience
	fmt.Printf("Site URL: http://%s:%d\n", site, rtConfig.Ports.Web)

	return nil
}

func getDefaultSiteForBrowse(benchPath string) string {
	// Try currentsite.txt first
	currentSitePath := filepath.Join(benchPath, "sites", "currentsite.txt")
	if data, err := os.ReadFile(currentSitePath); err == nil {
		return strings.TrimSpace(string(data))
	}

	// Try sites directory
	sitesDir := filepath.Join(benchPath, "sites")
	entries, err := os.ReadDir(sitesDir)
	if err != nil {
		return ""
	}

	for _, e := range entries {
		if e.IsDir() && e.Name() != "assets" && e.Name()[0] != '.' {
			return e.Name()
		}
	}

	return ""
}
