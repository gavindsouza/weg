package app

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/apps"
	"github.com/gavindsouza/weg/internal/completion"
	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var (
	forceRemove bool
	yesRemove   bool
)

var removeCmd = &cobra.Command{
	Use:     "remove <app-name>",
	Aliases: []string{"rm", "uninstall"},
	Short:   "Remove an app",
	Long: `Remove an app from the current project.

This uninstalls the app and removes it from the configuration.

Examples:
  weg app remove erpnext
  weg app rm erpnext -y`,
	Args:              cobra.ExactArgs(1),
	RunE:              runRemove,
	ValidArgsFunction: completion.CompleteInstalledAppNames,
}

func init() {
	removeCmd.Flags().BoolVarP(&forceRemove, "force", "f", false, "Force removal without confirmation")
	removeCmd.Flags().BoolVarP(&yesRemove, "yes", "y", false, "Skip confirmation prompt")
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
		return fmt.Errorf("not a weg-managed project")
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
	if !forceRemove && !yesRemove {
		fmt.Printf("Remove app %s? This cannot be undone. [y/N]: ", appName)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	fmt.Printf("Removing %s...\n", appName)

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

	fmt.Printf("Successfully removed %s\n", appName)
	fmt.Println("Note: Remember to remove the app from weg.toml if desired")

	return nil
}
