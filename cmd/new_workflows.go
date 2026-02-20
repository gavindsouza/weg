package cmd

import (
	"os"
	"path/filepath"
)

// createGitHubWorkflows creates CI and linter workflows
func createGitHubWorkflows(targetPath, moduleName, version string) error {
	workflowsDir := filepath.Join(targetPath, ".github", "workflows")
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		return err
	}

	// CI workflow
	ciWorkflow := tmplReplace("ci.yml", map[string]string{
		"MODULE_NAME": moduleName,
	})
	if err := os.WriteFile(filepath.Join(workflowsDir, "ci.yml"), []byte(ciWorkflow), 0644); err != nil {
		return err
	}

	// Linter workflow with semgrep
	if err := os.WriteFile(filepath.Join(workflowsDir, "linters.yml"), []byte(tmpl("linters.yml")), 0644); err != nil {
		return err
	}

	return nil
}
