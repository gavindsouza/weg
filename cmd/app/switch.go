package app

import (
	"fmt"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/apps"
	"github.com/gavindsouza/weg/internal/completion"
	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var switchCmd = &cobra.Command{
	Use:   "switch <app-name> <branch>",
	Short: "Switch an app to a different branch",
	Long: `Switch an installed app to a different git branch.

This checks out the specified branch and reinstalls dependencies.

Examples:
  weg app switch frappe version-15
  weg app switch erpnext develop`,
	Args:              cobra.ExactArgs(2),
	RunE:              runSwitch,
	ValidArgsFunction: completion.CompleteAppNamesForArg(0),
}

func runSwitch(cmd *cobra.Command, args []string) error {
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
	branch := args[1]

	var benchPath, appsDir string
	switch result.Context {
	case config.ContextWegBench:
		benchPath = absPath
		appsDir = filepath.Join(benchPath, "apps")
	case config.ContextWegApp:
		benchPath = filepath.Join(absPath, ".weg")
		appsDir = filepath.Join(benchPath, "apps")
	default:
		return fmt.Errorf("not a weg-managed project")
	}

	appPath := filepath.Join(appsDir, appName)

	// Check if app is installed
	if !apps.IsGitRepo(appPath) {
		return fmt.Errorf("app %s is not installed", appName)
	}

	// Get current branch
	currentBranch, err := apps.GetCurrentBranch(appPath)
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	if currentBranch == branch {
		fmt.Printf("%s is already on branch %s\n", appName, branch)
		return nil
	}

	fmt.Printf("Switching %s from %s to %s...\n", appName, currentBranch, branch)

	// Checkout the new branch
	if err := apps.Checkout(appPath, branch); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", branch, err)
	}

	// Reinstall dependencies
	opts := apps.InstallOptions{
		BenchPath: benchPath,
		AppsDir:   appsDir,
		Verbose:   true,
	}

	fmt.Println("Reinstalling dependencies...")
	if err := apps.InstallPythonDeps(appPath, opts); err != nil {
		fmt.Printf("Warning: failed to install Python deps: %v\n", err)
	}
	if err := apps.InstallNodeDeps(appPath, opts); err != nil {
		// Node deps optional
	}

	// Update state
	st, err := state.Load(absPath)
	if err != nil {
		st = state.NewState()
	}

	if appState, ok := st.Apps[appName]; ok {
		appState.Branch = branch
		st.Apps[appName] = appState
		if err := st.Save(absPath); err != nil {
			fmt.Printf("Warning: failed to save state: %v\n", err)
		}
	}

	fmt.Printf("Successfully switched %s to %s\n", appName, branch)
	fmt.Println("Note: Remember to update weg.toml and run 'weg build' if needed")

	return nil
}
