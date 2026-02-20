package site

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/completion"
	"github.com/gavindsouza/weg/internal/config"
	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/prompt"
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

	result, err := config.DetectContext(absPath)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	siteName := args[0]

	var benchPath string
	switch result.Context {
	case config.ContextWegBench:
		benchPath = absPath
	case config.ContextWegApp:
		benchPath = filepath.Join(absPath, ".weg")
	default:
		return wegerrors.NotInProject(absPath)
	}

	sitesDir := filepath.Join(benchPath, "sites")
	sitePath := filepath.Join(sitesDir, siteName)

	// Check if site exists
	if _, err := os.Stat(sitePath); os.IsNotExist(err) {
		return fmt.Errorf("site %s does not exist", siteName)
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

	// Build bench drop-site command
	cmdArgs := []string{"drop-site", siteName, "--force"}

	if dropRootPass != "" {
		cmdArgs = append(cmdArgs, "--db-root-password", dropRootPass)
	}

	if archived {
		cmdArgs = append(cmdArgs, "--archived-sites-path", filepath.Join(sitesDir, "archived"))
	} else {
		cmdArgs = append(cmdArgs, "--no-backup")
	}

	// Run bench drop-site
	benchCmd := exec.Command("bench", cmdArgs...)
	benchCmd.Dir = benchPath
	benchCmd.Stdout = os.Stdout
	benchCmd.Stderr = os.Stderr

	if err := benchCmd.Run(); err != nil {
		// Try manual removal if bench command fails
		output.Warning("bench drop-site failed, attempting manual removal...")
		if err := os.RemoveAll(sitePath); err != nil {
			return fmt.Errorf("failed to remove site directory: %w", err)
		}
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
