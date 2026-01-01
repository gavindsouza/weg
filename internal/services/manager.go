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

// Start starts all services using process-compose
func (m *Manager) Start() error {
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
	composePath := filepath.Join(m.BenchPath, "process-compose.yaml")

	cmd := exec.Command("process-compose", "ps", "-f", composePath)
	cmd.Dir = m.BenchPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// Logs shows logs from services
func (m *Manager) Logs(service string, follow bool) error {
	composePath := filepath.Join(m.BenchPath, "process-compose.yaml")

	args := []string{"logs", "-f", composePath}
	if follow {
		args = append(args, "--follow")
	}
	if service != "" {
		args = append(args, service)
	}

	cmd := exec.Command("process-compose", args...)
	cmd.Dir = m.BenchPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

// Restart restarts a specific service or all services
func (m *Manager) Restart(service string) error {
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
