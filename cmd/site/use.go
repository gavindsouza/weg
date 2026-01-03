package site

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/completion"
	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var useCmd = &cobra.Command{
	Use:   "use <site-name>",
	Short: "Set the default site",
	Long: `Set a site as the default for bench commands.

The default site is used when no --site flag is provided.

Examples:
  weg site use mysite.localhost`,
	Args:              cobra.ExactArgs(1),
	RunE:              runUse,
	ValidArgsFunction: completion.CompleteSiteNamesForArg(0),
}

func runUse(cmd *cobra.Command, args []string) error {
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
		return fmt.Errorf("not a weg-managed project")
	}

	sitesDir := filepath.Join(benchPath, "sites")
	sitePath := filepath.Join(sitesDir, siteName)

	// Check if site exists
	if _, err := os.Stat(sitePath); os.IsNotExist(err) {
		return fmt.Errorf("site %s does not exist", siteName)
	}

	// Update state
	st, err := state.Load(absPath)
	if err != nil {
		st = state.NewState()
	}

	// Add site to state if not present
	if !st.HasSite(siteName) {
		st.AddSite(state.SiteState{
			Name: siteName,
		})
	}

	st.SetDefaultSite(siteName)

	if err := st.Save(absPath); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	// Also update currentsite.txt for bench compatibility
	currentSitePath := filepath.Join(sitesDir, "currentsite.txt")
	if err := os.WriteFile(currentSitePath, []byte(siteName), 0644); err != nil {
		fmt.Printf("Warning: failed to update currentsite.txt: %v\n", err)
	}

	fmt.Printf("Default site set to %s\n", siteName)
	return nil
}
