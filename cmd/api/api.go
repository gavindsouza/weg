package api

import (
	"fmt"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

// Shared flags for all api subcommands
var (
	apiSite string
	apiUser string
	apiRaw  bool
)

// ApiCmd is the parent command for all api operations
var ApiCmd = &cobra.Command{
	Use:   "api",
	Short: "Make API calls to Frappe sites",
	Long: `Execute Frappe API calls directly without HTTP.

This command provides REST-style subcommands for document CRUD operations
and a generic 'call' command for invoking any whitelisted method.

Commands execute as Administrator by default. Use --user to change.

Examples:
  weg api get User                    # List all Users
  weg api get User/Administrator      # Get specific document
  weg api call frappe.ping            # Call a method
  weg api post User -d '{"email":"test@example.com"}'`,
}

func init() {
	// Persistent flags available to all subcommands
	ApiCmd.PersistentFlags().StringVarP(&apiSite, "site", "s", "", "Target site (default: auto-detect)")
	ApiCmd.PersistentFlags().StringVarP(&apiUser, "user", "u", "Administrator", "Execute as user")
	ApiCmd.PersistentFlags().BoolVar(&apiRaw, "raw", false, "Output raw JSON without formatting")

	// Register subcommands
	ApiCmd.AddCommand(getCmd)
	ApiCmd.AddCommand(postCmd)
	ApiCmd.AddCommand(putCmd)
	ApiCmd.AddCommand(deleteCmd)
	ApiCmd.AddCommand(callCmd)
}

// detectBenchAndSite finds the bench path and default site
func detectBenchAndSite() (benchPath, site string, err error) {
	absPath, err := filepath.Abs(".")
	if err != nil {
		return "", "", fmt.Errorf("invalid path: %w", err)
	}

	result, err := config.DetectContext(absPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to detect context: %w", err)
	}

	switch result.Context {
	case config.ContextWegApp:
		benchPath = filepath.Join(absPath, ".weg")
	case config.ContextWegBench:
		benchPath = absPath
	default:
		return "", "", fmt.Errorf("not a weg-managed project. Run 'weg init' first")
	}

	// Get site from flag or auto-detect
	site = apiSite
	if site == "" {
		// Try to get default site from state
		st, err := state.Load(absPath)
		if err == nil {
			site = st.GetDefaultSite()
		}

		// Fallback: scan sites directory
		if site == "" {
			site, _ = findFirstSite(benchPath)
		}
	}

	if site == "" {
		return "", "", fmt.Errorf("no site found. Create one with 'weg site new'")
	}

	return benchPath, site, nil
}

// findFirstSite returns the first site found in the sites directory
func findFirstSite(benchPath string) (string, error) {
	sitesDir := filepath.Join(benchPath, "sites")
	entries, err := filepath.Glob(filepath.Join(sitesDir, "*", "site_config.json"))
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		dir := filepath.Dir(entry)
		name := filepath.Base(dir)
		if name != "assets" {
			return name, nil
		}
	}
	return "", fmt.Errorf("no sites found")
}
