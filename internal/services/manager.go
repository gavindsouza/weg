package services

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// Manager handles service lifecycle
type Manager struct {
	BenchPath string
	Verbose   bool
}

// NewManager creates a new service manager
func NewManager(benchPath string) *Manager {
	return &Manager{
		BenchPath: benchPath,
	}
}

// isDevboxProject checks if this is a devbox-managed project
func (m *Manager) isDevboxProject() bool {
	devboxJson := filepath.Join(m.BenchPath, "devbox.json")
	_, err := os.Stat(devboxJson)
	return err == nil
}

// Start starts all services using devbox services or process-compose
func (m *Manager) Start() error {
	if m.isDevboxProject() {
		return m.startWithDevbox()
	}
	return m.startWithProcessCompose()
}

// startWithDevbox starts services using devbox services + process-compose
func (m *Manager) startWithDevbox() error {
	// First, ensure mariadb and redis are running via devbox services
	startMariadb := exec.Command("devbox", "services", "start", "mariadb", "-c", m.BenchPath)
	startMariadb.Dir = m.BenchPath
	if output, err := startMariadb.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to start mariadb: %w\n%s", err, string(output))
	}

	startRedis := exec.Command("devbox", "services", "start", "redis", "-c", m.BenchPath)
	startRedis.Dir = m.BenchPath
	if output, err := startRedis.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to start redis: %w\n%s", err, string(output))
	}

	// Then run process-compose via devbox for the Frappe services
	composePath := filepath.Join(m.BenchPath, "process-compose.yaml")
	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		return fmt.Errorf("process-compose.yaml not found. Run 'weg sync' first")
	}

	// Build the command: devbox run -- process-compose up -f process-compose.yaml
	devboxPath, err := exec.LookPath("devbox")
	if err != nil {
		return fmt.Errorf("devbox not found. Install it from https://www.jetify.com/devbox")
	}

	// Use exec to replace current process (allows Ctrl+C handling)
	args := []string{"devbox", "run", "-c", m.BenchPath, "--", "process-compose", "up", "-f", composePath}
	if err := syscall.Exec(devboxPath, args, os.Environ()); err != nil {
		// Fallback to normal execution if exec fails
		cmd := exec.Command(devboxPath, "run", "-c", m.BenchPath, "--", "process-compose", "up", "-f", composePath)
		cmd.Dir = m.BenchPath
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		return cmd.Run()
	}

	return nil
}

// startWithProcessCompose starts services using process-compose directly
func (m *Manager) startWithProcessCompose() error {
	composePath := filepath.Join(m.BenchPath, "process-compose.yaml")

	// Check if process-compose.yaml exists
	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		return fmt.Errorf("process-compose.yaml not found. Run 'weg sync' first")
	}

	// Check if process-compose is installed
	pcPath, err := exec.LookPath("process-compose")
	if err != nil {
		return fmt.Errorf("process-compose not found. Install it with: devbox add process-compose")
	}

	// Start process-compose
	cmd := exec.Command(pcPath, "up", "-f", composePath)
	cmd.Dir = m.BenchPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	// Use exec to replace current process (allows Ctrl+C handling)
	if err := syscall.Exec(pcPath, []string{"process-compose", "up", "-f", composePath}, os.Environ()); err != nil {
		// Fallback to normal execution if exec fails
		return cmd.Run()
	}

	return nil
}

// StartDetached starts services in the background
func (m *Manager) StartDetached() error {
	if m.isDevboxProject() {
		// Start mariadb and redis via devbox services
		startMariadb := exec.Command("devbox", "services", "start", "mariadb", "-c", m.BenchPath)
		startMariadb.Dir = m.BenchPath
		if output, err := startMariadb.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to start mariadb: %w\n%s", err, string(output))
		}

		startRedis := exec.Command("devbox", "services", "start", "redis", "-c", m.BenchPath)
		startRedis.Dir = m.BenchPath
		if output, err := startRedis.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to start redis: %w\n%s", err, string(output))
		}

		// Start Frappe services via process-compose in background
		composePath := filepath.Join(m.BenchPath, "process-compose.yaml")
		cmd := exec.Command("devbox", "run", "-c", m.BenchPath, "--", "process-compose", "up", "-f", composePath, "-d")
		cmd.Dir = m.BenchPath
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to start services: %w\n%s", err, string(output))
		}
		return nil
	}

	composePath := filepath.Join(m.BenchPath, "process-compose.yaml")

	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		return fmt.Errorf("process-compose.yaml not found. Run 'weg sync' first")
	}

	cmd := exec.Command("process-compose", "up", "-f", composePath, "-d")
	cmd.Dir = m.BenchPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start services: %w\n%s", err, string(output))
	}

	return nil
}

// Stop stops all running services
func (m *Manager) Stop() error {
	if m.isDevboxProject() {
		// Stop process-compose first
		composePath := filepath.Join(m.BenchPath, "process-compose.yaml")
		pcCmd := exec.Command("devbox", "run", "-c", m.BenchPath, "--", "process-compose", "down", "-f", composePath)
		pcCmd.Dir = m.BenchPath
		pcCmd.CombinedOutput() // Ignore errors, may not be running

		// Stop devbox services
		cmd := exec.Command("devbox", "services", "stop", "-c", m.BenchPath)
		cmd.Dir = m.BenchPath
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to stop services: %w\n%s", err, string(output))
		}
		return nil
	}

	composePath := filepath.Join(m.BenchPath, "process-compose.yaml")

	cmd := exec.Command("process-compose", "down", "-f", composePath)
	cmd.Dir = m.BenchPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to stop services: %w\n%s", err, string(output))
	}

	return nil
}

// Status shows the status of running services
func (m *Manager) Status() error {
	if m.isDevboxProject() {
		cmd := exec.Command("devbox", "services", "ls", "-c", m.BenchPath)
		cmd.Dir = m.BenchPath
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	composePath := filepath.Join(m.BenchPath, "process-compose.yaml")

	cmd := exec.Command("process-compose", "ps", "-f", composePath)
	cmd.Dir = m.BenchPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// Logs shows logs from services
func (m *Manager) Logs(service string, follow bool) error {
	// devbox services doesn't have a logs command, but uses process-compose under the hood
	// We can use process-compose directly since devbox services uses it
	composePath := filepath.Join(m.BenchPath, "process-compose.yaml")

	args := []string{"logs", "-f", composePath}
	if follow {
		args = append(args, "--follow")
	}
	if service != "" {
		args = append(args, service)
	}

	var cmd *exec.Cmd
	if m.isDevboxProject() {
		// Run process-compose via devbox to get proper environment
		devboxArgs := append([]string{"run", "-c", m.BenchPath, "--", "process-compose"}, args...)
		cmd = exec.Command("devbox", devboxArgs...)
	} else {
		cmd = exec.Command("process-compose", args...)
	}
	cmd.Dir = m.BenchPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

// Restart restarts a specific service or all services
func (m *Manager) Restart(service string) error {
	if m.isDevboxProject() {
		args := []string{"services", "restart", "-c", m.BenchPath}
		if service != "" {
			args = append(args, service)
		}
		cmd := exec.Command("devbox", args...)
		cmd.Dir = m.BenchPath
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to restart: %w\n%s", err, string(output))
		}
		return nil
	}

	composePath := filepath.Join(m.BenchPath, "process-compose.yaml")

	args := []string{"restart", "-f", composePath}
	if service != "" {
		args = append(args, service)
	}

	cmd := exec.Command("process-compose", args...)
	cmd.Dir = m.BenchPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to restart: %w\n%s", err, string(output))
	}

	return nil
}

// IsRunning checks if services are currently running
func (m *Manager) IsRunning() bool {
	if m.isDevboxProject() {
		cmd := exec.Command("devbox", "services", "ls", "-c", m.BenchPath)
		cmd.Dir = m.BenchPath
		output, err := cmd.Output()
		if err != nil {
			return false
		}
		// Check if output indicates running services
		return len(output) > 0
	}

	composePath := filepath.Join(m.BenchPath, "process-compose.yaml")

	cmd := exec.Command("process-compose", "ps", "-f", composePath, "-o", "json")
	cmd.Dir = m.BenchPath

	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// If we get valid JSON output with processes, they're running
	return len(output) > 2 // More than just "[]"
}
