package services

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// Manager handles service lifecycle
type Manager struct {
	BenchPath          string
	Verbose            bool
	ProcessComposePort int
	RunID              string // Run ID for identifying processes to kill
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

	// Run process-compose via devbox
	args := []string{"run", "-c", m.BenchPath, "--", "process-compose", "up", "-f", composePath}
	if m.ProcessComposePort > 0 {
		args = append(args, "-p", fmt.Sprintf("%d", m.ProcessComposePort))
	}
	cmd := exec.Command("devbox", args...)
	cmd.Dir = m.BenchPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
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
	args := []string{"up", "-f", composePath}
	if m.ProcessComposePort > 0 {
		args = append(args, "-p", fmt.Sprintf("%d", m.ProcessComposePort))
	}
	cmd := exec.Command(pcPath, args...)
	cmd.Dir = m.BenchPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
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
		args := []string{"run", "-c", m.BenchPath, "--", "process-compose", "up", "-f", composePath, "-D", "-t=false"}
		if m.ProcessComposePort > 0 {
			args = append(args, "-p", fmt.Sprintf("%d", m.ProcessComposePort))
		}
		cmd := exec.Command("devbox", args...)
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

	args := []string{"up", "-f", composePath, "-D", "-t=false"}
	if m.ProcessComposePort > 0 {
		args = append(args, "-p", fmt.Sprintf("%d", m.ProcessComposePort))
	}
	cmd := exec.Command("process-compose", args...)
	cmd.Dir = m.BenchPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start services: %w\n%s", err, string(output))
	}

	return nil
}

// Stop stops all running services
func (m *Manager) Stop() error {
	return m.StopWithTimeout(0, false)
}

// StopFast stops all services with a short timeout and SIGKILL for unresponsive processes
func (m *Manager) StopFast() error {
	return m.StopWithTimeout(3, true)
}

// StopWithTimeout stops services with custom timeout and optional SIGKILL
func (m *Manager) StopWithTimeout(timeoutSecs int, forceKill bool) error {
	composePath := filepath.Join(m.BenchPath, "process-compose.yaml")

	// Build process-compose down command with options
	pcArgs := []string{"down", "-f", composePath}
	if timeoutSecs > 0 {
		pcArgs = append(pcArgs, fmt.Sprintf("--timeout=%d", timeoutSecs))
	}
	if forceKill {
		pcArgs = append(pcArgs, "-k") // Send SIGKILL instead of SIGTERM
	}

	if m.isDevboxProject() {
		// Stop process-compose first
		devboxArgs := append([]string{"run", "-c", m.BenchPath, "--", "process-compose"}, pcArgs...)
		pcCmd := exec.Command("devbox", devboxArgs...)
		pcCmd.Dir = m.BenchPath
		pcCmd.CombinedOutput() // Ignore errors, may not be running

		// Stop devbox services
		cmd := exec.Command("devbox", "services", "stop", "-c", m.BenchPath)
		cmd.Dir = m.BenchPath
		cmd.CombinedOutput() // Ignore errors

		// Kill any orphaned processes from this bench
		m.killOrphanedProcesses()

		return nil
	}

	cmd := exec.Command("process-compose", pcArgs...)
	cmd.Dir = m.BenchPath
	cmd.CombinedOutput() // Ignore errors

	// Kill any orphaned processes
	m.killOrphanedProcesses()

	return nil
}

// killOrphanedProcesses kills any remaining frappe processes for this bench
func (m *Manager) killOrphanedProcesses() {
	// Kill by RunID if available (precise matching via environment)
	if m.RunID != "" {
		m.killByRunID()
	}

	// Kill orphaned child processes (e.g. esbuild, yarn) that may have been
	// reparented to init and lost their WEG_RUNNER env var.
	// Uses /proc cmdline scanning instead of pkill -f to avoid matching
	// unrelated processes with similar path substrings.
	m.killByPathPatterns()
}

// killByRunID kills all processes with WEG_RUNNER=<runID> in their environment
func (m *Manager) killByRunID() {
	pattern := fmt.Sprintf("WEG_RUNNER=%s", m.RunID)

	m.forEachProc(func(pid string) {
		envData, err := os.ReadFile(fmt.Sprintf("/proc/%s/environ", pid))
		if err != nil {
			return
		}
		if containsNullDelimited(envData, pattern) {
			syscall.Kill(atoiUnsafe(pid), syscall.SIGTERM)
		}
	})
}

// killByPathPatterns kills processes whose cmdline contains bench-specific paths
func (m *Manager) killByPathPatterns() {
	patterns := []string{
		filepath.Join(m.BenchPath, "sites"),                    // gunicorn, bench commands
		filepath.Join(m.BenchPath, "apps/frappe/socketio.js"),  // socketio
		filepath.Join(m.BenchPath, "apps/frappe/node_modules"), // esbuild, yarn
		filepath.Join(m.BenchPath, ".devbox"),                  // devbox-spawned node/yarn
		filepath.Join(m.BenchPath, "env"),                      // workers, scheduler, watch
	}

	m.forEachProc(func(pid string) {
		cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%s/cmdline", pid))
		if err != nil {
			return
		}
		// cmdline is null-delimited; join to match against full command
		cmd := string(bytes.ReplaceAll(cmdline, []byte{0}, []byte(" ")))
		for _, pattern := range patterns {
			if strings.Contains(cmd, pattern) {
				syscall.Kill(atoiUnsafe(pid), syscall.SIGTERM)
				return
			}
		}
	})
}

// forEachProc iterates over numeric /proc entries (PIDs)
func (m *Manager) forEachProc(fn func(pid string)) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid := entry.Name()
		if len(pid) == 0 || pid[0] < '0' || pid[0] > '9' {
			continue
		}
		fn(pid)
	}
}

// containsNullDelimited checks if null-delimited data contains a matching entry
func containsNullDelimited(data []byte, pattern string) bool {
	for _, entry := range bytes.Split(data, []byte{0}) {
		if bytes.HasPrefix(entry, []byte(pattern)) {
			return true
		}
	}
	return false
}

// atoiUnsafe converts a string of digits to int (assumes valid PID)
func atoiUnsafe(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
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
