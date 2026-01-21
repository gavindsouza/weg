package bench

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/prompt"
	"github.com/spf13/cobra"
)

var dropCmd = &cobra.Command{
	Use:     "drop <path>",
	Aliases: []string{"rm", "delete"},
	Short:   "Remove a bench",
	Long: `Remove a weg-managed bench.

This permanently deletes the bench directory and all its contents.
Use with caution!

Examples:
  weg bench drop ~/old-bench
  weg bench drop ./my-bench --force`,
	Args: cobra.ExactArgs(1),
	RunE: runDrop,
}

var dropForce bool

func init() {
	dropCmd.Flags().BoolVar(&dropForce, "force", false, "Skip confirmation")
}

func runDrop(cmd *cobra.Command, args []string) error {
	benchPath, err := filepath.Abs(args[0])
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Check if path exists
	if _, err := os.Stat(benchPath); os.IsNotExist(err) {
		return fmt.Errorf("path does not exist: %s", benchPath)
	}

	// Verify it's a bench
	result, err := config.DetectContext(benchPath)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	switch result.Context {
	case config.ContextWegBench, config.ContextBench:
		// OK - it's a bench
	case config.ContextWegApp:
		// It's an app with .weg - ask if they want to remove just .weg
		return handleAppWithWeg(benchPath)
	default:
		return fmt.Errorf("path is not a bench: %s", benchPath)
	}

	// Load bench info for display
	benchName := filepath.Base(benchPath)
	wegTomlPath := filepath.Join(benchPath, "weg.toml")
	if cfg, err := config.ParseWegToml(wegTomlPath); err == nil {
		benchName = cfg.Bench.Name
	}

	// Confirm
	if !dropForce {
		fmt.Printf("This will permanently delete bench '%s' at:\n", benchName)
		fmt.Printf("  %s\n\n", benchPath)

		// List what will be deleted
		appsDir := filepath.Join(benchPath, "apps")
		if entries, err := os.ReadDir(appsDir); err == nil && len(entries) > 0 {
			fmt.Printf("Apps to be deleted (%d):\n", len(entries))
			for _, e := range entries {
				if e.IsDir() {
					fmt.Printf("  - %s\n", e.Name())
				}
			}
			fmt.Println()
		}

		sitesDir := filepath.Join(benchPath, "sites")
		if entries, err := os.ReadDir(sitesDir); err == nil {
			var sites []string
			for _, e := range entries {
				if e.IsDir() && e.Name() != "assets" {
					sites = append(sites, e.Name())
				}
			}
			if len(sites) > 0 {
				fmt.Printf("Sites to be deleted (%d):\n", len(sites))
				for _, s := range sites {
					fmt.Printf("  - %s\n", s)
				}
				fmt.Println()
			}
		}

		if !prompt.Confirm("Are you sure?") {
			output.Print("Cancelled.")
			return nil
		}
	}

	output.Infof("Removing bench %s...", benchName)

	// Remove the directory
	if err := os.RemoveAll(benchPath); err != nil {
		return fmt.Errorf("failed to remove bench: %w", err)
	}

	output.Successf("Bench '%s' removed successfully", benchName)
	return nil
}

func handleAppWithWeg(appPath string) error {
	wegPath := filepath.Join(appPath, ".weg")

	if !dropForce {
		output.Print("This is a Frappe app with a .weg development environment.")
		output.Print("")
		output.Print("Options:")
		output.Print("  1. Remove only .weg/ (keep the app)")
		output.Print("  2. Remove everything (app + .weg)")
		output.Print("  3. Cancel")
		output.Print("")
		fmt.Fprint(prompt.Writer, "Choose [1/2/3]: ")

		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(answer)

		switch answer {
		case "1":
			output.Info("Removing .weg/ directory...")
			if err := os.RemoveAll(wegPath); err != nil {
				return fmt.Errorf("failed to remove .weg: %w", err)
			}
			output.Success(".weg/ removed. App preserved.")
			return nil

		case "2":
			if !prompt.ConfirmDanger("Remove entire app directory?") {
				output.Print("Cancelled.")
				return nil
			}
			output.Info("Removing entire app directory...")
			if err := os.RemoveAll(appPath); err != nil {
				return fmt.Errorf("failed to remove app: %w", err)
			}
			output.Success("App and .weg/ removed.")
			return nil

		default:
			output.Print("Cancelled.")
			return nil
		}
	}

	// Force mode - just remove .weg
	if err := os.RemoveAll(wegPath); err != nil {
		return fmt.Errorf("failed to remove .weg: %w", err)
	}
	output.Success(".weg/ removed.")
	return nil
}
