package app

import (
	"fmt"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/apps"
	"github.com/gavindsouza/weg/internal/completion"
	"github.com/gavindsouza/weg/internal/config"
	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/prompt"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var (
	forceRemove bool
)

var removeCmd = &cobra.Command{
	Use:     "remove <app-name>",
	Aliases: []string{"rm", "uninstall"},
	Short:   "Remove an app",
	Long: `Remove an app from the current project.

This uninstalls the app and removes it from the configuration.

Examples:
  weg app remove erpnext
  weg app rm erpnext --force`,
	Args:              cobra.ExactArgs(1),
	RunE:              runRemove,
	ValidArgsFunction: completion.CompleteInstalledAppNames,
}

func init() {
	removeCmd.Flags().BoolVar(&forceRemove, "force", false, "Skip confirmation prompt")
}

func runRemove(cmd *cobra.Command, args []string) error {
	path := "."
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	result, err := config.DetectContext(absPath)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	appName := args[0]

	// Prevent removing frappe
	if appName == "frappe" {
		return fmt.Errorf("cannot remove frappe - it is required")
	}

	var benchPath, appsDir string
	switch result.Context {
	case config.ContextWegBench:
		benchPath = absPath
		appsDir = filepath.Join(benchPath, "apps")
	case config.ContextWegApp:
		benchPath = filepath.Join(absPath, ".weg")
		appsDir = filepath.Join(benchPath, "apps")
		// Can't remove the main app
		if appName == filepath.Base(absPath) {
			return fmt.Errorf("cannot remove the main app")
		}
	default:
		return wegerrors.NotInProject(absPath)
	}

	// Load state
	st, err := state.Load(absPath)
	if err != nil {
		st = state.NewState()
	}

	if !st.HasApp(appName) {
		return fmt.Errorf("app %s is not installed", appName)
	}

	// Confirm
	if !forceRemove && !prompt.ConfirmDanger("Remove app %s?", appName) {
		output.Print("Cancelled.")
		return nil
	}

	output.Infof("Removing %s...", appName)

	// Remove the app
	opts := apps.InstallOptions{
		BenchPath: benchPath,
		AppsDir:   appsDir,
	}

	if err := apps.RemoveApp(appName, opts); err != nil {
		return fmt.Errorf("failed to remove app: %w", err)
	}

	// Update state
	st.RemoveApp(appName)
	if err := st.Save(absPath); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	output.Successf("Removed %s", appName)
	output.Info("Note: Remember to remove the app from weg.toml if desired")

	return nil
}
