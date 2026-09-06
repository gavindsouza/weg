package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/prompt"
)

// bootstrapPlan is the target state the wizard converges toward.
type bootstrapPlan struct {
	appName    string
	moduleName string
	versions   []string
	databases  []string
	devVersion string
	devDB      string
	siteName   string
}

const bootstrapTagline = "Frappe version compatibility and development environment settings"

func bootstrapApp(absPath string, result *config.DetectionResult) error {
	appName := result.AppName
	if name, err := config.ProjectName(absPath); err == nil {
		appName = name
	}
	PrintInfo("")
	PrintInfo("Frappe app detected: %s", appName)
	PrintInfo("")
	PrintInfo("weg will migrate this project in 5 steps. You'll be asked to confirm each one.")
	PrintInfo("Where a file already exists you'll see a diff first.")
	PrintInfo("")

	plan := detectPlan(absPath, appName)
	if err := confirmCompatibility(plan); err != nil {
		return err
	}

	if err := ensureWegGitignored(absPath); err != nil {
		return err
	}

	envReady := false
	if err := stepAppConfig(absPath, plan); err != nil {
		return err
	}
	if err := stepScaffoldAI(absPath); err != nil {
		return err
	}
	if err := stepScaffoldPrecommit(absPath); err != nil {
		return err
	}
	if err := stepWorkflows(absPath, plan); err != nil {
		return err
	}
	if err := stepDevelopmentEnvironment(absPath, plan); err != nil {
		return err
	}
	envReady = true

	printBootstrapSummary(plan, envReady)
	return nil
}

// ensureWegGitignored adds .weg/ to .gitignore so the machine-local bench stays
// out of version control. It also covers apps installed into .weg/apps.
func ensureWegGitignored(absPath string) error {
	gitignorePath := filepath.Join(absPath, ".gitignore")
	content, err := os.ReadFile(gitignorePath)
	if os.IsNotExist(err) {
		return os.WriteFile(gitignorePath, []byte(".weg/\n"), 0644)
	}
	if err != nil {
		return err
	}
	if strings.Contains(string(content), ".weg") {
		return nil
	}
	updated := strings.TrimRight(string(content), "\n") + "\n\n.weg/\n"
	return os.WriteFile(gitignorePath, []byte(updated), 0644)
}

// bootstrapWegApp handles an app that already has [tool.weg]. The wizard runs
// again but with replace semantics (default No) so re-running it is safe.
func bootstrapWegApp(absPath string, result *config.DetectionResult) error {
	appName := result.AppName
	if name, err := config.ProjectName(absPath); err == nil {
		appName = name
	}
	PrintInfo("")
	PrintInfo("Weg-managed app detected: %s", appName)
	PrintInfo("")
	PrintInfo("This project already has [tool.weg]. You can still regenerate the")
	PrintInfo("scaffold and CI workflows, or re-sync the development environment.")
	PrintInfo("")

	plan, err := existingPlan(absPath, appName)
	if err != nil {
		return err
	}

	if err := ensureWegGitignored(absPath); err != nil {
		return err
	}

	envReady := false
	if err := stepAppConfig(absPath, plan); err != nil {
		return err
	}
	if err := stepScaffoldAI(absPath); err != nil {
		return err
	}
	if err := stepScaffoldPrecommit(absPath); err != nil {
		return err
	}
	if err := stepWorkflows(absPath, plan); err != nil {
		return err
	}
	if err := stepDevelopmentEnvironment(absPath, plan); err != nil {
		return err
	}
	envReady = true

	printBootstrapSummary(plan, envReady)
	return nil
}

// detectPlan builds the target config from signals already in the repo
// (existing CI matrix), falling back to sane defaults.
func detectPlan(absPath, appName string) *bootstrapPlan {
	plan := &bootstrapPlan{
		appName:    appName,
		moduleName: toModuleName(appName),
		siteName:   fmt.Sprintf("%s.localhost", toModuleName(appName)),
	}

	versions, databases := detectCICompatibility(absPath)
	if len(versions) > 0 {
		plan.versions = versions
	} else {
		plan.versions = []string{"15"}
	}
	if len(databases) > 0 {
		plan.databases = databases
	} else {
		plan.databases = []string{"mariadb"}
	}
	plan.devVersion = plan.versions[0]
	plan.devDB = plan.databases[0]
	return plan
}

// existingPlan reads the current [tool.weg] so re-running the wizard doesn't
// lose the declared compatibility matrix.
func existingPlan(absPath, appName string) (*bootstrapPlan, error) {
	plan := &bootstrapPlan{
		appName:    appName,
		moduleName: toModuleName(appName),
		siteName:   fmt.Sprintf("%s.localhost", toModuleName(appName)),
	}

	appConfig, err := config.ParsePyproject(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pyproject.toml: %w", err)
	}
	plan.versions = appConfig.Compatibility.Frappe
	plan.databases = appConfig.Compatibility.Databases
	plan.devVersion = plan.versions[0]
	plan.devDB = plan.databases[0]
	return plan, nil
}

// confirmCompatibility lets the user review or override the detected matrix.
func confirmCompatibility(plan *bootstrapPlan) error {
	if !AssumeYes() && prompt.ConfirmDefault(true,
		"Compatibility: frappe %s, database %s. Correct?",
		strings.Join(plan.versions, ", "), strings.Join(plan.databases, ", ")) {
		return nil
	}

	versionsInput, err := prompt.Input(
		"Frappe versions (comma-separated, e.g. 15,16,develop): ")
	if err != nil {
		return err
	}
	if v := parseCSV(versionsInput); len(v) > 0 {
		plan.versions = v
	}

	databasesInput, err := prompt.Input(
		"Databases (comma-separated, e.g. mariadb,postgres): ")
	if err != nil {
		return err
	}
	if v := parseCSV(databasesInput); len(v) > 0 {
		plan.databases = v
	}

	cfg := &config.AppConfig{
		Compatibility: config.CompatibilityConfig{Frappe: plan.versions, Databases: plan.databases},
		Dev:           config.DevConfig{Frappe: plan.devVersion, Database: plan.devDB},
	}
	if err := config.ValidateAppConfig(cfg); err != nil {
		return fmt.Errorf("invalid compatibility config: %w", err)
	}

	plan.devVersion = plan.versions[0]
	plan.devDB = plan.databases[0]
	return nil
}

func parseCSV(input string) []string {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(input, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// stepAppConfig writes the [tool.weg] section into pyproject.toml.
func stepAppConfig(absPath string, plan *bootstrapPlan) error {
	pyprojectPath := filepath.Join(absPath, "pyproject.toml")
	content, err := os.ReadFile(pyprojectPath)
	if err != nil {
		content = []byte{}
	}

	section := buildWegSection(plan)
	existing := findWegSection(string(content))

	var newContent string
	if existing != "" {
		newContent = replaceWegSection(string(content), section)
		if output.DiffEmpty(string(content), newContent) {
			PrintInfo("  [tool.weg] is already up to date.")
			return nil
		}
		PrintInfo("\nDiff for pyproject.toml ([tool.weg]):")
		printIndentedDiff(output.Diff(existing, section))
		if !prompt.ConfirmDefault(false, "Replace [tool.weg] with the new configuration?") {
			PrintInfo("  Skipping.")
			return nil
		}
	} else {
		newContent = addWegSection(string(content), section)
		if output.DiffEmpty(string(content), newContent) {
			PrintInfo("  [tool.weg] is already up to date.")
			return nil
		}
		PrintInfo("\nPreview for pyproject.toml (+[tool.weg]):")
		printIndentedDiff(output.Diff(string(content), newContent))
		if !prompt.ConfirmDefault(true, "Add [tool.weg] compatibility config to pyproject.toml?") {
			PrintInfo("  Skipping.")
			return nil
		}
	}

	if err := os.WriteFile(pyprojectPath, []byte(newContent), 0644); err != nil {
		return errors.Config(filepath.Base(pyprojectPath), "write", err)
	}
	PrintInfo("  Updated pyproject.toml")
	return nil
}

// stepScaffoldAI creates CLAUDE.md and frappe.review.md.
func stepScaffoldAI(absPath string) error {
	claudePath := filepath.Join(absPath, "CLAUDE.md")
	existing, err := os.ReadFile(claudePath)
	if err == nil {
		newContent := tmpl("claude.md")
		if output.DiffEmpty(string(existing), newContent) {
			PrintInfo("  CLAUDE.md already matches weg's version.")
			return nil
		}
		PrintInfo("\nDiff for CLAUDE.md:")
		printIndentedDiff(output.Diff(string(existing), newContent))
		if !prompt.ConfirmDefault(false, "Replace CLAUDE.md with weg's version?") {
			PrintInfo("  Skipping.")
			return nil
		}
		return writeScaffoldFiles(absPath)
	}

	PrintInfo("\nSteps 2-3 create agent and lint tooling:")
	if !prompt.ConfirmDefault(true, "Scaffold AI agent files (CLAUDE.md, .claude/commands)?") {
		PrintInfo("  Skipping.")
		return nil
	}
	return writeScaffoldFiles(absPath)
}

// stepScaffoldPrecommit adds the pre-commit config with Frappe semgrep rules.
func stepScaffoldPrecommit(absPath string) error {
	configPath := filepath.Join(absPath, ".pre-commit-config.yaml")
	existing, err := os.ReadFile(configPath)
	if err == nil {
		newContent := tmpl("scaffold-precommit.yaml")
		if output.DiffEmpty(string(existing), newContent) {
			PrintInfo("  Pre-commit config already matches weg's version.")
			return nil
		}
		PrintInfo("\nDiff for .pre-commit-config.yaml:")
		printIndentedDiff(output.Diff(string(existing), newContent))
		if !prompt.ConfirmDefault(false, "Replace pre-commit config with weg's version?") {
			PrintInfo("  Skipping.")
			return nil
		}
		return writeFile(filepath.Join(absPath, ".pre-commit-config.yaml"), tmpl("scaffold-precommit.yaml"))
	}

	if !prompt.ConfirmDefault(true, "Add pre-commit config (.pre-commit-config.yaml)?") {
		PrintInfo("  Skipping.")
		return nil
	}
	return writeFile(filepath.Join(absPath, ".pre-commit-config.yaml"), tmpl("scaffold-precommit.yaml"))
}

func writeScaffoldFiles(absPath string) error {
	if err := writeFile(filepath.Join(absPath, "CLAUDE.md"), tmpl("claude.md")); err != nil {
		return err
	}
	return writeFile(filepath.Join(absPath, ".claude", "commands", "frappe.review.md"), tmpl("frappe-review.md"))
}

func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

// stepWorkflows generates .github/workflows/ci.yml and linters.yml.
func stepWorkflows(absPath string, plan *bootstrapPlan) error {
	ciPath := filepath.Join(absPath, ".github", "workflows", "ci.yml")
	ciOld, _ := os.ReadFile(ciPath)
	ciNew, err := renderCIWorkflow(plan)
	if err != nil {
		return err
	}

	lintersPath := filepath.Join(absPath, ".github", "workflows", "linters.yml")
	lintersOld, _ := os.ReadFile(lintersPath)
	lintersNew := tmpl("linters.yml")

	if output.DiffEmpty(string(ciOld), ciNew) && output.DiffEmpty(string(lintersOld), lintersNew) {
		PrintInfo("  Workflows already at target state.")
		return nil
	}

	if len(ciOld) == 0 && len(lintersOld) == 0 {
		if !prompt.ConfirmDefault(true, "Generate GitHub Actions workflows (ci.yml, linters.yml)?") {
			PrintInfo("  Skipping.")
			return nil
		}
	} else {
		if len(ciOld) > 0 && !output.DiffEmpty(string(ciOld), ciNew) {
			PrintInfo("\nDiff for .github/workflows/ci.yml:")
			printIndentedDiff(output.Diff(string(ciOld), ciNew))
		}
		if len(lintersOld) > 0 && !output.DiffEmpty(string(lintersOld), lintersNew) {
			PrintInfo("\nDiff for .github/workflows/linters.yml:")
			printIndentedDiff(output.Diff(string(lintersOld), lintersNew))
		}
		if !prompt.ConfirmDefault(false, "Replace existing workflows with weg's versions?") {
			PrintInfo("  Skipping.")
			return nil
		}
	}

	if err := createGitHubWorkflows(absPath, plan.moduleName, plan.versions); err != nil {
		return errors.Config(".github/workflows", "write", err)
	}
	PrintInfo("  Generated .github/workflows")
	return nil
}

func renderCIWorkflow(plan *bootstrapPlan) (string, error) {
	return renderCIFrom(plan.moduleName, plan.versions)
}

// renderCIFrom renders the ci.yml workflow template for a module and its
// supported Frappe versions.
func renderCIFrom(moduleName string, versions []string) (string, error) {
	matrix, err := buildMatrixInclude(versions)
	if err != nil {
		return "", err
	}
	content := tmplReplace("ci.yml", map[string]string{
		"MATRIX_INCLUDE": strings.TrimRight(matrix, "\n"),
		"MODULE_NAME":    moduleName,
	})
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return content, nil
}

// stepDevelopmentEnvironment creates the hidden .weg bench and site via the
// same pipeline 'weg sync' uses.
func stepDevelopmentEnvironment(absPath string, plan *bootstrapPlan) error {
	wegDir := filepath.Join(absPath, ".weg")

	if _, err := os.Stat(filepath.Join(wegDir, "apps")); err == nil {
		PrintInfo("\nDevelopment environment already exists at .weg/")
		if prompt.ConfirmDefault(false, "Re-sync it now (reinstall apps, sites)?") {
			if err := ensureEnvironment(wegDir, plan.devVersion); err != nil {
				return fmt.Errorf("failed to re-initialize environment: %w", err)
			}
			return syncAppWithWegToml(absPath, wegDir, filepath.Join(wegDir, "weg.toml"))
		}
		PrintInfo("  Skipping.")
		return nil
	}

	PrintInfo("\nDevelopment environment setup:")
	PrintInfo("  .weg/        hidden bench on this machine")
	PrintInfo("  devbox       python/node/mariadb runtime (devbox install)")
	PrintInfo("  site         %s with your app installed", plan.siteName)
	if !prompt.ConfirmDefault(true, "Set up the development environment now? This installs Frappe and takes a few minutes.") {
		PrintInfo("  Skipping. Next: 'weg sync' when you're ready.")
		return nil
	}

	if _, err := exec.LookPath("devbox"); err != nil {
		return fmt.Errorf("devbox is required to set up the environment; install it from https://jetpack.io/devbox then run 'weg sync'")
	}

	for _, dir := range []string{"apps", "sites", "config", "config/pids", "logs"} {
		if err := os.MkdirAll(filepath.Join(wegDir, dir), 0755); err != nil {
			return errors.Config(filepath.Join(".weg", dir), "create", err)
		}
	}

	wegTomlContent, err := renderWegToml(plan)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(wegDir, "weg.toml"), []byte(wegTomlContent), 0644); err != nil {
		return errors.Config(".weg/weg.toml", "write", err)
	}

	// Initialize the devbox runtime + venv before sync, so bench has a working
	// Python environment to install apps and create the site. ensureEnvironment
	// is idempotent (skips if devbox.json already exists).
	if err := ensureEnvironment(wegDir, plan.devVersion); err != nil {
		return fmt.Errorf("failed to initialize development environment: %w", err)
	}

	// The wizard already confirmed the env setup, so the sync pipeline's own
	// "apply these changes?" confirm should not re-ask.
	assumeYes := prompt.AssumeYes
	prompt.AssumeYes = true
	syncErr := syncAppWithWegToml(absPath, wegDir, filepath.Join(wegDir, "weg.toml"))
	prompt.AssumeYes = assumeYes
	if syncErr != nil {
		return fmt.Errorf("failed to set up environment: %w", syncErr)
	}

	siteDir := filepath.Join(wegDir, "sites", plan.siteName)
	if _, err := os.Stat(siteDir); err != nil {
		PrintInfo("  Environment was not set up (sync had no changes to apply or was cancelled).")
		PrintInfo("  Run 'weg sync' to retry.")
		return nil
	}
	PrintInfo("  Development environment ready at .weg/ (site %s).", plan.siteName)
	return nil
}

func renderWegToml(plan *bootstrapPlan) (string, error) {
	return tmplReplace("weg.toml", map[string]string{
		"APP_NAME":       plan.appName,
		"VERSION":        plan.devVersion,
		"DATABASE":       plan.devDB,
		"MODULE_NAME":    plan.moduleName,
		"SITE_NAME":      plan.siteName,
		"FRAPPE_BRANCH":  frappeBranch(plan.devVersion),
	}), nil
}

// bootstrapBench imports a traditional bench into weg management.
func bootstrapBench(absPath string, result *config.DetectionResult) error {
	PrintInfo("")
	PrintInfo("Traditional bench detected: %s", absPath)
	if !prompt.ConfirmDefault(false, "Import this bench into weg management?") {
		return nil
	}
	if err := initBench(absPath, result); err != nil {
		return err
	}
	if err := stepScaffoldAI(absPath); err != nil {
		return err
	}
	if err := stepScaffoldPrecommit(absPath); err != nil {
		return err
	}
	PrintInfo("")
	PrintInfo("Bench imported. Run 'weg sync' to fold apps and sites into the weg-managed state.")
	return nil
}

// bootstrapWegBench handles a bench already managed by weg.
func bootstrapWegBench(absPath string, result *config.DetectionResult) error {
	PrintInfo("")
	PrintInfo("Weg-managed bench detected: %s", absPath)
	PrintInfo("Nothing to migrate here; this bench is already managed by weg.")
	return nil
}

func printBootstrapSummary(plan *bootstrapPlan, envReady bool) {
	if !envReady {
		return
	}
	PrintInfo("")
	PrintInfo("Bootstrap complete for %s", plan.appName)
	PrintInfo("  Config:    [tool.weg] supports frappe %s · %s",
		strings.Join(plan.versions, ", "), strings.Join(plan.databases, ", "))
	PrintInfo("  Scaffold:  CLAUDE.md, .claude/")
	PrintInfo("  Workflows: .github/workflows/ci.yml, linters.yml")
	PrintInfo("")
	PrintInfo("Next steps:")
	PrintInfo("  weg start               # Start the development server")
	PrintInfo("  weg test                # Run tests against %s", plan.devVersion)
	PrintInfo("  git add . && git commit # Review the changes and commit")
}

func printIndentedDiff(diffContent string) {
	for _, line := range strings.Split(strings.TrimSuffix(diffContent, "\n"), "\n") {
		PrintInfo("    %s", line)
	}
}