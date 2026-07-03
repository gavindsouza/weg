package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/spf13/cobra"
)

var scaffoldCmd = &cobra.Command{
	Use:   "scaffold [type]",
	Short: "Scaffold development tooling into your project",
	Long: `Add development tooling and AI agent configurations to your Frappe project.

Available scaffolds:
  ai          Add CLAUDE.md and AI agent skills for Frappe development
  precommit   Add pre-commit configuration with Frappe semgrep rules
  all         Add all available scaffolds

Examples:
  weg scaffold ai          # Add AI agent configuration
  weg scaffold precommit   # Add pre-commit hooks
  weg scaffold all         # Add everything

When to use which: 'weg scaffold' adds tooling to an existing weg project;
it does not create projects — see 'weg new', 'weg create', or 'weg init'
for that.`,
	Args:              cobra.ExactArgs(1),
	RunE:              runScaffold,
	SilenceUsage:      true,
	ValidArgsFunction: scaffoldCompletion,
}

var scaffoldForce bool

func init() {
	rootCmd.AddCommand(scaffoldCmd)
	scaffoldCmd.Flags().BoolVarP(&scaffoldForce, "force", "f", false, "Overwrite existing files")
}

func scaffoldCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return []string{"ai", "precommit", "all"}, cobra.ShellCompDirectiveNoFileComp
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}

func runScaffold(cmd *cobra.Command, args []string) error {
	scaffoldType := args[0]

	path := "."
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Detect context - should be a weg-managed app
	result, err := config.DetectProjectContext(absPath)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	if result.Context != config.ContextWegApp && result.Context != config.ContextApp {
		return errors.Usage("scaffold should be run from a Frappe app directory")
	}

	switch scaffoldType {
	case "ai":
		return scaffoldAI(absPath)
	case "precommit":
		return scaffoldPrecommit(absPath)
	case "all":
		if err := scaffoldAI(absPath); err != nil {
			return err
		}
		return scaffoldPrecommit(absPath)
	default:
		return errors.Validation("type", fmt.Sprintf("unknown scaffold type: %s. Use 'ai', 'precommit', or 'all'", scaffoldType))
	}
}

func scaffoldAI(projectPath string) error {
	output.Print("Scaffolding AI agent configuration...")

	// Create CLAUDE.md
	claudePath := filepath.Join(projectPath, "CLAUDE.md")
	if err := writeFileIfNotExists(claudePath, tmpl("claude.md")); err != nil {
		return err
	}
	output.Printf("  Created %s", claudePath)

	// Create .claude/commands directory
	commandsDir := filepath.Join(projectPath, ".claude", "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		return fmt.Errorf("failed to create commands directory: %w", err)
	}

	// Create frappe.review skill
	reviewPath := filepath.Join(commandsDir, "frappe.review.md")
	if err := writeFileIfNotExists(reviewPath, tmpl("frappe-review.md")); err != nil {
		return err
	}
	output.Printf("  Created %s", reviewPath)

	output.Print("AI agent configuration complete!")
	return nil
}

func scaffoldPrecommit(projectPath string) error {
	output.Print("Scaffolding pre-commit configuration...")

	configPath := filepath.Join(projectPath, ".pre-commit-config.yaml")
	if err := writeFileIfNotExists(configPath, tmpl("scaffold-precommit.yaml")); err != nil {
		return err
	}
	output.Printf("  Created %s", configPath)

	output.Print("\nPre-commit configuration complete!")
	output.Print("Run these commands to activate:")
	output.Print("  pip install pre-commit")
	output.Print("  pre-commit install")
	return nil
}

func writeFileIfNotExists(path, content string) error {
	if !scaffoldForce {
		if _, err := os.Stat(path); err == nil {
			output.Printf("  Skipping %s (already exists, use --force to overwrite)", path)
			return nil
		}
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	return os.WriteFile(path, []byte(content), 0644)
}
