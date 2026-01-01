package bench

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/config"
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
	dropCmd.Flags().BoolVarP(&dropForce, "force", "f", false, "Skip confirmation")
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

		fmt.Print("Are you sure? This cannot be undone. [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	fmt.Printf("Removing bench %s...\n", benchName)

	// Remove the directory
	if err := os.RemoveAll(benchPath); err != nil {
		return fmt.Errorf("failed to remove bench: %w", err)
	}

	fmt.Printf("✓ Bench '%s' removed successfully\n", benchName)
	return nil
}

func handleAppWithWeg(appPath string) error {
	wegPath := filepath.Join(appPath, ".weg")

	if !dropForce {
		fmt.Println("This is a Frappe app with a .weg development environment.")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  1. Remove only .weg/ (keep the app)")
		fmt.Println("  2. Remove everything (app + .weg)")
		fmt.Println("  3. Cancel")
		fmt.Println()
		fmt.Print("Choose [1/2/3]: ")

		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(answer)

		switch answer {
		case "1":
			fmt.Println("Removing .weg/ directory...")
			if err := os.RemoveAll(wegPath); err != nil {
				return fmt.Errorf("failed to remove .weg: %w", err)
			}
			fmt.Println("✓ .weg/ removed. App preserved.")
			return nil

		case "2":
			fmt.Print("Confirm removal of entire app directory? [y/N]: ")
			confirm, _ := reader.ReadString('\n')
			confirm = strings.TrimSpace(strings.ToLower(confirm))
			if confirm != "y" && confirm != "yes" {
				fmt.Println("Cancelled.")
				return nil
			}
			fmt.Println("Removing entire app directory...")
			if err := os.RemoveAll(appPath); err != nil {
				return fmt.Errorf("failed to remove app: %w", err)
			}
			fmt.Println("✓ App and .weg/ removed.")
			return nil

		default:
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// Force mode - just remove .weg
	if err := os.RemoveAll(wegPath); err != nil {
		return fmt.Errorf("failed to remove .weg: %w", err)
	}
	fmt.Println("✓ .weg/ removed.")
	return nil
}
