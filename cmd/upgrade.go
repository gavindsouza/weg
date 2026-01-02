package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/tools"
	"github.com/spf13/cobra"
)

var upgradeCmd = &cobra.Command{
	Use:          "upgrade",
	Short:        "Upgrade Frappe to next version",
	Long:         `Upgrade the Frappe framework to the next major version.

Version progression: 14 → 15 → 16 → develop

This command handles the full upgrade process:
  1. Updates weg.toml configuration
  2. Regenerates devbox environment (Python, Node, etc.)
  3. Checks out the new branch for frappe
  4. Reinstalls all Python dependencies
  5. Reinstalls all Node dependencies
  6. Runs database migrations

Examples:
  weg upgrade            # Upgrade to next version (e.g., 15 → 16)
  weg upgrade --hierarchyTip Shows current → next without upgrading`,
	Args:         cobra.NoArgs,
	RunE:         runUpgrade,
	SilenceUsage: true,
}

var (
	noMigrate bool
)

func init() {
	rootCmd.AddCommand(upgradeCmd)
	upgradeCmd.Flags().BoolVar(&noMigrate, "no-migrate", false, "Skip database migrations")
}

// getNextVersion returns the next Frappe version in the upgrade path
func getNextVersion(current string) (string, error) {
	switch current {
	case "14":
		return "15", nil
	case "15":
		return "16", nil
	case "16":
		return "develop", nil
	case "develop":
		return "", fmt.Errorf("already on develop (bleeding edge)")
	default:
		return "", fmt.Errorf("unknown version '%s'", current)
	}
}

func runUpgrade(cmd *cobra.Command, args []string) error {
	// Detect context
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	result, err := config.DetectContext(cwd)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	var benchPath string
	var configPath string

	switch result.Context {
	case config.ContextWegApp:
		benchPath = filepath.Join(cwd, ".weg")
		configPath = filepath.Join(benchPath, "weg.toml")
	case config.ContextWegBench:
		benchPath = cwd
		configPath = filepath.Join(cwd, "weg.toml")
	default:
		return fmt.Errorf("not a weg-managed project. Run 'weg init' first")
	}

	// Load current config
	benchConfig, err := config.ParseWegToml(benchPath)
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	currentVersion := benchConfig.Frappe.Version

	// Determine next version automatically
	targetVersion, err := getNextVersion(currentVersion)
	if err != nil {
		return err
	}

	// Get version info for comparison
	currentInfo, err := tools.GetFrappeVersion(tools.NormalizeFrappeVersion(currentVersion))
	if err != nil {
		return fmt.Errorf("unknown current version: %w", err)
	}

	targetInfo, err := tools.GetFrappeVersion(tools.NormalizeFrappeVersion(targetVersion))
	if err != nil {
		return fmt.Errorf("unknown target version: %w", err)
	}

	// Show upgrade plan
	fmt.Printf("\nUpgrading from Frappe %s to %s\n\n", currentVersion, targetVersion)
	fmt.Println("Environment changes:")
	if currentInfo.PythonVersion != targetInfo.PythonVersion {
		fmt.Printf("  Python: %s → %s\n", currentInfo.PythonVersion, targetInfo.PythonVersion)
	}
	if currentInfo.NodeVersion != targetInfo.NodeVersion {
		fmt.Printf("  Node: %s → %s\n", currentInfo.NodeVersion, targetInfo.NodeVersion)
	}
	if currentInfo.PackageManager != targetInfo.PackageManager {
		fmt.Printf("  Package manager: %s → %s\n", currentInfo.PackageManager, targetInfo.PackageManager)
	}
	fmt.Println()

	fmt.Println("This will:")
	fmt.Println("  1. Update weg.toml configuration")
	fmt.Println("  2. Regenerate devbox environment")
	fmt.Println("  3. Checkout", targetVersion, "branch for frappe")
	fmt.Println("  4. Reinstall all Python dependencies")
	fmt.Println("  5. Reinstall all Node dependencies")
	if !noMigrate {
		fmt.Println("  6. Run database migrations")
	}
	fmt.Println()

	// Confirm unless -y
	if !yes {
		fmt.Print("Continue? [y/N] ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" {
			PrintInfo("Upgrade cancelled")
			return nil
		}
	}

	// Step 1: Update weg.toml
	PrintInfo("[1/6] Updating configuration...")
	if err := updateWegTomlVersion(configPath, targetVersion); err != nil {
		return fmt.Errorf("failed to update config: %w", err)
	}

	// Step 2: Regenerate devbox
	PrintInfo("[2/6] Regenerating devbox environment...")
	if err := regenerateDevbox(benchPath, targetInfo); err != nil {
		return fmt.Errorf("failed to regenerate devbox: %w", err)
	}

	// Step 3: Checkout frappe branch
	PrintInfo("[3/6] Checking out %s branch for frappe...", targetVersion)
	frappePath := filepath.Join(benchPath, "apps", "frappe")
	if err := checkoutFrappeBranch(frappePath, tools.NormalizeFrappeVersion(targetVersion)); err != nil {
		return fmt.Errorf("failed to checkout frappe: %w", err)
	}

	// Step 4: Reinstall Python deps
	PrintInfo("[4/6] Installing Python dependencies...")
	if err := reinstallPythonDeps(benchPath); err != nil {
		return fmt.Errorf("failed to install Python deps: %w", err)
	}

	// Step 5: Reinstall Node deps
	PrintInfo("[5/6] Installing Node dependencies...")
	if err := reinstallNodeDeps(benchPath, targetInfo.PackageManager); err != nil {
		PrintVerbose("Warning: Node deps install failed: %v", err)
		// Continue anyway - node deps are optional
	}

	// Step 6: Run migrations
	if !noMigrate {
		PrintInfo("[6/6] Running database migrations...")
		if err := runMigrations(benchPath, result); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	} else {
		PrintInfo("[6/6] Skipping database migrations (--no-migrate)")
	}

	fmt.Println()
	PrintInfo("Upgrade complete! Run 'weg start' to start services.")
	return nil
}

// updateWegTomlVersion updates the frappe version in weg.toml
func updateWegTomlVersion(configPath, version string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	content := string(data)
	branch := tools.NormalizeFrappeVersion(version)

	// Update [frappe] version
	lines := strings.Split(content, "\n")
	var newLines []string
	inFrappeSection := false
	inAppsFrappeSection := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track sections
		if strings.HasPrefix(trimmed, "[frappe]") {
			inFrappeSection = true
			inAppsFrappeSection = false
		} else if strings.HasPrefix(trimmed, "[apps.frappe]") {
			inFrappeSection = false
			inAppsFrappeSection = true
		} else if strings.HasPrefix(trimmed, "[") {
			inFrappeSection = false
			inAppsFrappeSection = false
		}

		// Update version in [frappe] section
		if inFrappeSection && strings.HasPrefix(trimmed, "version") {
			line = fmt.Sprintf("version = \"%s\"", version)
		}

		// Update branch in [apps.frappe] section
		if inAppsFrappeSection && strings.HasPrefix(trimmed, "branch") {
			line = fmt.Sprintf("branch = \"%s\"", branch)
		}

		newLines = append(newLines, line)
	}

	return os.WriteFile(configPath, []byte(strings.Join(newLines, "\n")), 0644)
}

// regenerateDevbox updates devbox.json with new packages for target version
func regenerateDevbox(benchPath string, target *tools.FrappeVersion) error {
	// Build package list from target version deps
	var packages []string
	for _, dep := range target.Dependencies {
		pkg := dep.Name
		if dep.Version != "" {
			pkg = fmt.Sprintf("%s@%s", dep.Name, dep.Version)
		}
		packages = append(packages, pkg)
	}

	// Always include these
	packages = append(packages, "uv", "process-compose")

	// Remove all existing packages and add new ones
	// First get current packages
	cmd := exec.Command("devbox", "rm", "--all", "-c", benchPath)
	cmd.Run() // Ignore errors

	// Add new packages
	args := append([]string{"add", "-c", benchPath}, packages...)
	cmd = exec.Command("devbox", args...)
	cmd.Dir = benchPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// checkoutFrappeBranch fetches and checks out a branch for frappe
func checkoutFrappeBranch(repoPath, branch string) error {
	// Fetch the branch
	fetchCmd := exec.Command("git", "fetch", "origin", branch)
	fetchCmd.Dir = repoPath
	if output, err := fetchCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch failed: %w\n%s", err, output)
	}

	// Checkout
	checkoutCmd := exec.Command("git", "checkout", branch)
	checkoutCmd.Dir = repoPath
	if output, err := checkoutCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git checkout failed: %w\n%s", err, output)
	}

	// Pull latest
	pullCmd := exec.Command("git", "pull", "--ff-only")
	pullCmd.Dir = repoPath
	pullCmd.CombinedOutput() // Ignore errors - might be detached

	return nil
}

// reinstallPythonDeps reinstalls all Python dependencies
func reinstallPythonDeps(benchPath string) error {
	appsDir := filepath.Join(benchPath, "apps")
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		return err
	}

	// Remove existing venv to force fresh install with new Python
	venvPath := filepath.Join(benchPath, ".venv")
	os.RemoveAll(venvPath)

	// Install each app
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		appPath := filepath.Join(appsDir, entry.Name())

		// Check if it's a Python package
		if _, err := os.Stat(filepath.Join(appPath, "pyproject.toml")); err != nil {
			if _, err := os.Stat(filepath.Join(appPath, "setup.py")); err != nil {
				continue
			}
		}

		PrintVerbose("  Installing %s...", entry.Name())
		cmd := exec.Command("devbox", "run", "-c", benchPath, "--", "uv", "pip", "install", "-e", appPath)
		cmd.Dir = benchPath
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to install %s: %w\n%s", entry.Name(), err, output)
		}
	}

	return nil
}

// reinstallNodeDeps reinstalls Node dependencies for all apps
func reinstallNodeDeps(benchPath, packageManager string) error {
	appsDir := filepath.Join(benchPath, "apps")
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		appPath := filepath.Join(appsDir, entry.Name())

		// Check if it has package.json
		if _, err := os.Stat(filepath.Join(appPath, "package.json")); err != nil {
			continue
		}

		PrintVerbose("  Installing node deps for %s...", entry.Name())

		var cmd *exec.Cmd
		switch packageManager {
		case "pnpm":
			cmd = exec.Command("devbox", "run", "-c", benchPath, "--", "pnpm", "install")
		case "bun":
			cmd = exec.Command("devbox", "run", "-c", benchPath, "--", "bun", "install")
		default: // yarn
			cmd = exec.Command("devbox", "run", "-c", benchPath, "--", "yarn", "install")
		}
		cmd.Dir = appPath
		cmd.CombinedOutput() // Ignore errors for node
	}

	return nil
}

// runMigrations runs bench migrate on all sites
func runMigrations(benchPath string, result *config.DetectionResult) error {
	sitesDir := filepath.Join(benchPath, "sites")
	entries, err := os.ReadDir(sitesDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "assets" || entry.Name()[0] == '.' {
			continue
		}

		siteName := entry.Name()
		PrintVerbose("  Migrating %s...", siteName)

		shellCmd := fmt.Sprintf("cd %s && ../.venv/bin/python -m frappe.utils.bench_helper frappe --site %s migrate",
			sitesDir, siteName)

		cmd := exec.Command("devbox", "run", "-c", benchPath, "--", "sh", "-c", shellCmd)
		cmd.Dir = benchPath
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("migration failed for %s: %w", siteName, err)
		}
	}

	return nil
}
