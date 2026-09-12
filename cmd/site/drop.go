package site

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/completion"
	"github.com/gavindsouza/weg/internal/config"
	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/prompt"
	"github.com/gavindsouza/weg/internal/services"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var (
	forceDrop    bool
	dropRootPass string
	archived     bool
)

var dropCmd = &cobra.Command{
	Use:     "drop <site-name>",
	Aliases: []string{"delete", "rm"},
	Short:   "Delete a site",
	Long: `Delete a Frappe site.

This drops the database and removes the site directory.

Examples:
  weg site drop mysite.localhost
  weg site drop mysite.localhost --force`,
	Args:              cobra.ExactArgs(1),
	RunE:              runDrop,
	ValidArgsFunction: completion.CompleteSiteNamesForArg(0),
}

func init() {
	dropCmd.Flags().BoolVar(&forceDrop, "force", false, "Skip confirmation")
	dropCmd.Flags().StringVar(&dropRootPass, "db-root-password", "", "Database root password")
	dropCmd.Flags().BoolVar(&archived, "archived", false, "Also remove archived sites")
}

func runDrop(cmd *cobra.Command, args []string) error {
	path := "."
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	result, err := config.DetectProjectContext(absPath)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	siteName := args[0]

	var benchPath string
	switch result.Context {
	case config.ContextWegBench:
		benchPath = result.BenchPath
	case config.ContextWegApp:
		benchPath = result.BenchPath
	default:
		return wegerrors.NotInProject(absPath)
	}

	sitesDir := filepath.Join(benchPath, "sites")
	sitePath := filepath.Join(sitesDir, siteName)

	// Check if site exists. One weg still tracks but whose directory is
	// gone only needs forgetting.
	if _, err := os.Stat(sitePath); os.IsNotExist(err) {
		st, loadErr := state.Load(absPath)
		if loadErr != nil || !st.HasSite(siteName) {
			return wegerrors.NotFound("site", siteName)
		}
		st.RemoveSite(siteName)
		if err := st.Save(absPath); err != nil {
			return fmt.Errorf("failed to save state: %w", err)
		}
		output.Infof("Site %s was already gone from disk; removed it from weg's state", siteName)
		return nil
	}

	// Confirm
	if !forceDrop {
		output.Printf("This will permanently delete site %s and its database.", siteName)
		if !prompt.Confirm("Are you sure?") {
			output.Print("Cancelled.")
			return nil
		}
	}

	output.Infof("Dropping site %s...", siteName)

	if dropRootPass == "" {
		dropRootPass = config.DBRootPassword(benchPath)
	}
	dropArgs := []string{"--no-backup"}
	if archived {
		dropArgs = []string{"--archived-sites-path", filepath.Join(sitesDir, "archived")}
	}
	if err := services.DropSite(benchPath, siteName, dropRootPass, dropArgs...); err != nil {
		return err
	}

	// Update state
	st, err := state.Load(absPath)
	if err != nil {
		st = state.NewState()
	}

	wasDefault := false
	if siteState, ok := st.Sites[siteName]; ok {
		wasDefault = siteState.DefaultSite
	}

	st.RemoveSite(siteName)

	// If this was default, set a new default
	if wasDefault && len(st.Sites) > 0 {
		for name := range st.Sites {
			st.SetDefaultSite(name)
			output.Infof("Set %s as new default site", name)
			break
		}
	}

	if err := st.Save(absPath); err != nil {
		output.Warningf("failed to save state: %v", err)
	}

	output.Successf("Dropped site %s", siteName)
	return nil
}
