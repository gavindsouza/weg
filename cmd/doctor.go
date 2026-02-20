package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/output"
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
  - Python environment

Examples:
  weg doctor       # Run all checks in current project
  weg doctor -v    # Run with verbose output`,
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
	result, err := config.DetectProjectContext(absPath)
	if err != nil {
		output.Print("[ ] Weg project")
		output.Print("    Not a weg-managed project. Run 'weg init' first.")
		return nil
	}

	var benchPath string
	switch result.Context {
	case config.ContextWegApp:
		benchPath = result.BenchPath
	case config.ContextWegBench:
		benchPath = result.BenchPath
	default:
		output.Print("[ ] Weg project")
		output.Print("    Not a weg-managed project. Run 'weg init' first.")
		return nil
	}

	output.Print("Weg Doctor")
	output.Print("==========")
	output.Print("")

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

	// Check workers
	checks = append(checks, checkWorkers(benchPath))

	// Print results
	issues := 0
	for _, c := range checks {
		if c.ok {
			output.Printf("[x] %s", c.name)
		} else {
			output.Printf("[ ] %s", c.name)
			issues++
		}
		if c.message != "" {
			output.Printf("    %s", c.message)
		}
	}

	output.Print("")
	if issues == 0 {
		output.Print("All checks passed!")
	} else {
		output.Printf("%d issue(s) found.", issues)
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
	venvPython := filepath.Join(benchPath, "env", "bin", "python")
	if _, err := os.Stat(venvPython); os.IsNotExist(err) {
		return checkResult{"Python venv", false, "env not found. Run 'devbox install' in the bench directory."}
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
	// Check for Unix socket files - these indicate services are running
	// Socket-based services don't have port conflicts with system services
	mariadbSocket := filepath.Join(benchPath, ".devbox", "virtenv", "mariadb", "run", "mysql.sock")
	redisSocket := filepath.Join(benchPath, ".devbox", "virtenv", "redis", "redis.sock")

	mariadbRunning := isSocketRunning(mariadbSocket)
	redisRunning := isSocketRunning(redisSocket)

	// Build detailed status for each service
	var lines []string
	allOk := true

	if mariadbRunning {
		lines = append(lines, "[x] MariaDB")
	} else {
		lines = append(lines, "[ ] MariaDB - not running")
		allOk = false
	}

	if redisRunning {
		lines = append(lines, "[x] Redis")
	} else {
		lines = append(lines, "[ ] Redis - not running")
		allOk = false
	}

	message := strings.Join(lines, "\n    ")
	if !allOk {
		message += "\n    Run 'weg start' to start services."
	}

	return checkResult{"Services", allOk, message}
}

// isSocketRunning checks if a Unix socket file exists and is actually a socket
func isSocketRunning(socketPath string) bool {
	info, err := os.Stat(socketPath)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSocket != 0
}

func checkRuntime(benchPath string) checkResult {
	rtConfig, err := runtime.Load(benchPath)
	if err != nil {
		return checkResult{"Runtime", false, "Services not started. Run 'weg start'."}
	}

	return checkResult{"Runtime", true, fmt.Sprintf("Web: %d, SocketIO: %d", rtConfig.Ports.Web, rtConfig.Ports.SocketIO)}
}

func checkWorkers(benchPath string) checkResult {
	benchConfig, err := config.ParseWegToml(benchPath)
	if err != nil {
		return checkResult{"Workers", false, "Cannot read weg.toml"}
	}

	workers := benchConfig.Services.Workers
	if len(workers) == 0 {
		return checkResult{"Workers", true, "1 worker (all queues) [default]"}
	}

	// Count total workers and summarize
	total := 0
	shared := 0
	dedicated := 0
	var queues []string

	for queue, count := range workers {
		if count <= 0 {
			continue
		}
		total += count
		if queue == "all" {
			shared += count
		} else {
			dedicated += count
			queues = append(queues, queue)
		}
	}

	msg := fmt.Sprintf("%d total: %d shared, %d dedicated", total, shared, dedicated)
	if len(queues) > 0 {
		msg += fmt.Sprintf(" (%s)", strings.Join(queues, ", "))
	}

	return checkResult{"Workers", true, msg}
}
