/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package workspace

import (
	"fmt"
	"os"
	"path/filepath"

	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/workspace"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Set up workspace with pre-commit hooks",
	Long: `Initialize the workspace with:
  - weg_workspace/ directory
  - .pre-commit-config.yaml with ruff, eslint, and weg collapse hook
  - Adds weg_workspace/ to .gitignore

This sets up a proper development environment for editing scripts.`,
	RunE: runInit,
}

func runInit(cmd *cobra.Command, args []string) error {
	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Check if we're in a weg clone
	if _, err := os.Stat(".weg"); os.IsNotExist(err) {
		return wegerrors.NotFound("remote clone", ".weg")
	}

	// Create workspace directory
	workspaceDir := filepath.Join(cwd, workspace.WorkspaceDir)
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		return fmt.Errorf("failed to create workspace directory: %w", err)
	}
	output.Printf("Created %s/", workspace.WorkspaceDir)

	// Update .gitignore
	if err := updateGitignore(cwd); err != nil {
		output.Warningf("failed to update .gitignore: %v", err)
	} else {
		output.Print("Updated .gitignore")
	}

	// Create pre-commit config if it doesn't exist
	precommitPath := filepath.Join(cwd, ".pre-commit-config.yaml")
	if _, err := os.Stat(precommitPath); os.IsNotExist(err) {
		if err := createPrecommitConfig(precommitPath); err != nil {
			output.Warningf("failed to create pre-commit config: %v", err)
		} else {
			output.Print("Created .pre-commit-config.yaml")
		}
	} else {
		output.Print("Skipped .pre-commit-config.yaml (already exists)")
	}

	output.Print("")
	output.Print("Workspace initialized! Next steps:")
	output.Print("  weg workspace expand    # Extract code files")
	output.Print("  pre-commit install      # Enable git hooks (optional)")

	return nil
}

func updateGitignore(baseDir string) error {
	gitignorePath := filepath.Join(baseDir, ".gitignore")

	// Read existing content
	content := ""
	if data, err := os.ReadFile(gitignorePath); err == nil {
		content = string(data)
	}

	// Check if already present
	entry := workspace.WorkspaceDir + "/"
	if contains(content, entry) {
		return nil
	}

	// Append
	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if content != "" && content[len(content)-1] != '\n' {
		f.WriteString("\n")
	}
	f.WriteString("\n# Expanded workspace (edit these, not JSON)\n")
	f.WriteString(entry + "\n")

	return nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[:len(substr)] == substr || contains(s[1:], substr)))
}

func createPrecommitConfig(path string) error {
	config := `# Pre-commit hooks for weg workspace
# Install with: pre-commit install

repos:
  # Collapse workspace before commit
  - repo: local
    hooks:
      - id: weg-workspace-collapse
        name: Collapse weg workspace
        entry: weg workspace collapse
        language: system
        pass_filenames: false
        files: ^weg_workspace/

  # Python linting with ruff
  - repo: https://github.com/astral-sh/ruff-pre-commit
    rev: v0.4.4
    hooks:
      - id: ruff
        files: ^weg_workspace/.*\.py$
        args: [--fix]
      - id: ruff-format
        files: ^weg_workspace/.*\.py$

  # JavaScript linting (optional - uncomment if you have eslint configured)
  # - repo: https://github.com/pre-commit/mirrors-eslint
  #   rev: v8.56.0
  #   hooks:
  #     - id: eslint
  #       files: ^weg_workspace/.*\.js$
  #       additional_dependencies:
  #         - eslint@8.56.0
`
	return os.WriteFile(path, []byte(config), 0644)
}
