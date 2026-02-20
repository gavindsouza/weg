package api

import (
	"fmt"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/config"
	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/remote"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

// Shared flags for all api subcommands
var (
	apiSite   string
	apiUser   string
	apiRaw    bool
	apiURL    string
	apiKey    string
	apiSecret string
)

// ApiCmd is the parent command for all api operations
var ApiCmd = &cobra.Command{
	Use:   "api",
	Short: "Make API calls to Frappe sites",
	Long: `Execute Frappe API calls to local or remote sites.

This command provides REST-style subcommands for document CRUD operations
and a generic 'call' command for invoking any whitelisted method.

Local mode (default): Executes directly via Python, as Administrator.
Remote mode (--url): Uses HTTP API with API key authentication.

Credential resolution for remote mode (highest priority first):
  1. CLI flags: --api-key and --api-secret
  2. Environment: WEG_API_KEY and WEG_API_SECRET
  3. Global config: ~/.config/weg/credentials.toml (keyed by hostname)

Examples:
  # Local site
  weg api get User                    # List all Users
  weg api get User/Administrator      # Get specific document
  weg api call frappe.ping            # Call a method

  # Remote site (explicit credentials)
  weg api -U https://site.frappe.cloud -k KEY -K SECRET get User

  # Remote site (credentials from env or config)
  export WEG_API_KEY=xxx WEG_API_SECRET=yyy
  weg api -U https://site.frappe.cloud get User`,
}

func init() {
	// Persistent flags available to all subcommands
	ApiCmd.PersistentFlags().StringVarP(&apiSite, "site", "s", "", "Target site (default: auto-detect)")
	ApiCmd.PersistentFlags().StringVarP(&apiUser, "user", "u", "Administrator", "Execute as user")
	ApiCmd.PersistentFlags().BoolVar(&apiRaw, "raw", false, "Output raw JSON without formatting")

	// Remote site flags
	ApiCmd.PersistentFlags().StringVarP(&apiURL, "url", "U", "", "Remote site URL (enables HTTP mode)")
	ApiCmd.PersistentFlags().StringVarP(&apiKey, "api-key", "k", "", "API key for remote auth")
	ApiCmd.PersistentFlags().StringVarP(&apiSecret, "api-secret", "K", "", "API secret for remote auth")

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

	result, err := config.DetectProjectContext(absPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to detect context: %w", err)
	}

	switch result.Context {
	case config.ContextWegApp:
		benchPath = result.BenchPath
	case config.ContextWegBench:
		benchPath = result.BenchPath
	default:
		return "", "", wegerrors.NotInProject(absPath)
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
		return "", "", wegerrors.Usage("no site found. Create one with 'weg site new'")
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
	return "", wegerrors.NotFound("sites", "")
}

// isRemoteMode returns true if --url flag is set
func isRemoteMode() bool {
	return apiURL != ""
}

// resolveRemoteCredentials resolves API credentials using the standard hierarchy:
// 1. CLI flags (--api-key, --api-secret)
// 2. Environment (WEG_API_KEY, WEG_API_SECRET)
// 3. Global config (~/.config/weg/credentials.toml by hostname)
func resolveRemoteCredentials() (key, secret string, err error) {
	// 1. CLI flags have highest priority
	if apiKey != "" && apiSecret != "" {
		return apiKey, apiSecret, nil
	}

	// 2. Use existing credential resolution (env → global config)
	siteHost := remote.ExtractHost(apiURL)
	creds, err := remote.LoadCredentialsForSite(".", siteHost)
	if err == nil && creds.Auth.APIKey != "" && creds.Auth.APISecret != "" {
		return creds.Auth.APIKey, creds.Auth.APISecret, nil
	}

	return "", "", fmt.Errorf("no credentials found for %s\n\nProvide via:\n  - Flags: --api-key and --api-secret\n  - Environment: WEG_API_KEY and WEG_API_SECRET\n  - Config: ~/.config/weg/credentials.toml", siteHost)
}
