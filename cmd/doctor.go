package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/runtime"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check environment health",
	Long: `Diagnose common issues with your weg environment.

Checks:
  - devbox installation and configuration
  - Required services (mariadb, redis)
  - Site configuration
  - Asset symlinks
  - Python environment`,
	RunE: runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

type checkResult struct {
	name    string
	ok      bool
	message string
}

func runDoctor(cmd *cobra.Command, args []string) error {
	path := "."
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Detect context
	result, err := config.DetectContext(absPath)
	if err != nil {
		fmt.Println("[ ] Weg project")
		fmt.Println("    Not a weg-managed project. Run 'weg init' first.")
		return nil
	}

	var benchPath string
	switch result.Context {
	case config.ContextWegApp:
		benchPath = filepath.Join(absPath, ".weg")
	case config.ContextWegBench:
		benchPath = absPath
	default:
		fmt.Println("[ ] Weg project")
		fmt.Println("    Not a weg-managed project. Run 'weg init' first.")
		return nil
	}

	fmt.Println("Weg Doctor")
	fmt.Println("==========")
	fmt.Println()

	checks := []checkResult{}

	// Check devbox
	checks = append(checks, checkDevbox(benchPath))

	// Check weg.toml
	checks = append(checks, checkWegToml(benchPath))

	// Check directories
	checks = append(checks, checkDirectories(benchPath))

	// Check Python environment
	checks = append(checks, checkPythonEnv(benchPath))

	// Check apps
	checks = append(checks, checkApps(benchPath))

	// Check sites
	checks = append(checks, checkSites(benchPath))

	// Check asset symlinks
	checks = append(checks, checkAssets(benchPath))

	// Check common_site_config.json
	checks = append(checks, checkSiteConfig(benchPath))

	// Check services
	checks = append(checks, checkServices(benchPath))

	// Check runtime
	checks = append(checks, checkRuntime(benchPath))

	// Print results
	issues := 0
	for _, c := range checks {
		if c.ok {
			fmt.Printf("[x] %s\n", c.name)
		} else {
			fmt.Printf("[ ] %s\n", c.name)
			issues++
		}
		if c.message != "" {
			fmt.Printf("    %s\n", c.message)
		}
	}

	fmt.Println()
	if issues == 0 {
		fmt.Println("All checks passed!")
	} else {
		fmt.Printf("%d issue(s) found.\n", issues)
	}

	return nil
}

func checkDevbox(benchPath string) checkResult {
	devboxJSON := filepath.Join(benchPath, "devbox.json")
	if _, err := os.Stat(devboxJSON); os.IsNotExist(err) {
		return checkResult{"Devbox", false, "devbox.json not found. Run 'weg sync' to initialize."}
	}

	// Check if devbox is installed
	if _, err := exec.LookPath("devbox"); err != nil {
		return checkResult{"Devbox", false, "devbox not installed. Visit https://www.jetify.com/devbox/docs/installing_devbox/"}
	}

	return checkResult{"Devbox", true, ""}
}

func checkWegToml(benchPath string) checkResult {
	wegToml := filepath.Join(benchPath, "weg.toml")
	if _, err := os.Stat(wegToml); os.IsNotExist(err) {
		return checkResult{"weg.toml", false, "Configuration file not found."}
	}

	_, err := config.ParseWegToml(benchPath)
	if err != nil {
		return checkResult{"weg.toml", false, fmt.Sprintf("Parse error: %v", err)}
	}

	return checkResult{"weg.toml", true, ""}
}

func checkDirectories(benchPath string) checkResult {
	dirs := []string{"apps", "sites", "logs", "config"}
	missing := []string{}

	for _, dir := range dirs {
		path := filepath.Join(benchPath, dir)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			missing = append(missing, dir)
		}
	}

	if len(missing) > 0 {
		return checkResult{"Directories", false, fmt.Sprintf("Missing: %v", missing)}
	}

	return checkResult{"Directories", true, ""}
}

func checkPythonEnv(benchPath string) checkResult {
	venvPython := filepath.Join(benchPath, ".venv", "bin", "python")
	if _, err := os.Stat(venvPython); os.IsNotExist(err) {
		return checkResult{"Python venv", false, ".venv not found. Run 'devbox install' in the bench directory."}
	}

	// Check if frappe is installed
	cmd := exec.Command(venvPython, "-c", "import frappe")
	if err := cmd.Run(); err != nil {
		return checkResult{"Python venv", false, "Frappe not installed in venv. Run 'weg sync'."}
	}

	return checkResult{"Python venv", true, ""}
}

func checkApps(benchPath string) checkResult {
	appsDir := filepath.Join(benchPath, "apps")
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		return checkResult{"Apps", false, "Cannot read apps directory."}
	}

	if len(entries) == 0 {
		return checkResult{"Apps", false, "No apps installed. Run 'weg sync'."}
	}

	// Check for frappe
	frappeDir := filepath.Join(appsDir, "frappe")
	if _, err := os.Stat(frappeDir); os.IsNotExist(err) {
		return checkResult{"Apps", false, "Frappe not installed. Run 'weg sync'."}
	}

	appCount := 0
	for _, e := range entries {
		if e.IsDir() {
			appCount++
		}
	}

	return checkResult{"Apps", true, fmt.Sprintf("%d app(s) installed", appCount)}
}

func checkSites(benchPath string) checkResult {
	sitesDir := filepath.Join(benchPath, "sites")
	entries, err := os.ReadDir(sitesDir)
	if err != nil {
		return checkResult{"Sites", false, "Cannot read sites directory."}
	}

	siteCount := 0
	for _, e := range entries {
		if e.IsDir() && e.Name() != "assets" && e.Name()[0] != '.' {
			siteCount++
		}
	}

	if siteCount == 0 {
		return checkResult{"Sites", false, "No sites created. Run 'weg sync'."}
	}

	return checkResult{"Sites", true, fmt.Sprintf("%d site(s) created", siteCount)}
}

func checkAssets(benchPath string) checkResult {
	assetsDir := filepath.Join(benchPath, "sites", "assets")
	if _, err := os.Stat(assetsDir); os.IsNotExist(err) {
		return checkResult{"Assets", false, "sites/assets directory not found."}
	}

	// Check for frappe assets symlink
	frappeAssets := filepath.Join(assetsDir, "frappe")
	info, err := os.Lstat(frappeAssets)
	if err != nil {
		return checkResult{"Assets", false, "Frappe assets not linked. Run 'weg sync'."}
	}

	if info.Mode()&os.ModeSymlink == 0 {
		return checkResult{"Assets", false, "Frappe assets is not a symlink."}
	}

	return checkResult{"Assets", true, ""}
}

func checkSiteConfig(benchPath string) checkResult {
	configPath := filepath.Join(benchPath, "sites", "common_site_config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return checkResult{"Site config", false, "common_site_config.json not found. Run 'weg sync'."}
	}

	return checkResult{"Site config", true, ""}
}

func checkServices(benchPath string) checkResult {
	// Check if devbox services are running
	cmd := exec.Command("devbox", "services", "ls", "-c", benchPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return checkResult{"Services", false, fmt.Sprintf("Cannot check devbox services: %v", err)}
	}

	outputStr := string(output)

	// Parse output line by line to check mariadb status
	mariadbRunning := false
	redisRunning := false
	for _, line := range strings.Split(outputStr, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			name := fields[0]
			status := fields[1]
			if name == "mariadb" && status == "Running" {
				mariadbRunning = true
			}
			if name == "redis" && status == "Running" {
				redisRunning = true
			}
		}
	}

	if !mariadbRunning {
		// Check if there's any output at all
		if len(strings.TrimSpace(outputStr)) == 0 {
			return checkResult{"Services", false, "No services output. Run 'weg start'."}
		}
		return checkResult{"Services", false, "MariaDB not running. Run 'weg start'."}
	}

	if redisRunning {
		return checkResult{"Services", true, "MariaDB and Redis running"}
	}
	return checkResult{"Services", true, "MariaDB running (Redis managed by devbox)"}
}

func checkRuntime(benchPath string) checkResult {
	rtConfig, err := runtime.Load(benchPath)
	if err != nil {
		return checkResult{"Runtime", false, "Services not started. Run 'weg start'."}
	}

	return checkResult{"Runtime", true, fmt.Sprintf("Web: %d, SocketIO: %d", rtConfig.Ports.Web, rtConfig.Ports.SocketIO)}
}
