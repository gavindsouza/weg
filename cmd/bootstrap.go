package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/errors"
	"github.com/spf13/cobra"
)

var bootstrapCmd = &cobra.Command{
	Use:   "bootstrap [path]",
	Short: "Migrate an existing project into weg management",
	Long: `Migrate an existing Frappe app or bench into a weg-managed project.

The bootstrap wizard walks through migrating an unmanaged project:

  1. Add [tool.weg] compatibility config (pyproject.toml)
  2. Scaffold AI agent files (CLAUDE.md, .claude/)
  3. Add pre-commit configuration
  4. Generate GitHub Actions workflows (CI + linters)
  5. Set up the local development environment

Where a file already exists you get to review a diff before replacing it.
Pass --yes to accept every step non-interactively.

Examples:
  weg bootstrap             Migrate the current directory
  weg bootstrap ./myapp     Migrate an app at path
  weg bootstrap --yes       Accept every step non-interactively`,
	Args:         cobra.MaximumNArgs(1),
	RunE:         runBootstrap,
	SilenceUsage: true,
}

func init() {
	rootCmd.AddCommand(bootstrapCmd)
}

func runBootstrap(cmd *cobra.Command, args []string) error {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	result, err := config.DetectContext(absPath)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	switch result.Context {
	case config.ContextFresh:
		return errors.Usage("nothing to migrate in an empty directory; use 'weg new' to start a project")
	case config.ContextApp:
		return bootstrapApp(absPath, result)
	case config.ContextWegApp:
		return bootstrapWegApp(absPath, result)
	case config.ContextBench:
		return bootstrapBench(absPath, result)
	case config.ContextWegBench:
		return bootstrapWegBench(absPath, result)
	default:
		return fmt.Errorf("unknown context")
	}
}
