package runtime

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
)

// GenerateRunID creates a unique ID for this run
func GenerateRunID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Config holds runtime state for a running weg environment
type Config struct {
	Ports Ports  `json:"ports"`
	PID   int    `json:"pid,omitempty"`
	RunID string `json:"run_id,omitempty"` // Unique ID for this run, used to identify processes
}

// Ports holds the actual ports being used by services
type Ports struct {
	Web       int `json:"web"`
	SocketIO  int `json:"socketio"`
	Redis     int `json:"redis"`
	// Process-compose API port
	ProcessCompose int `json:"process_compose"`
}

// DefaultPorts returns the preferred default ports
func DefaultPorts() Ports {
	return Ports{
		Web:            8000,
		SocketIO:       9000,
		Redis:          6379,
		ProcessCompose: 8080,
	}
}

// RuntimePath returns the path to runtime.json for a bench
func RuntimePath(benchPath string) string {
	return filepath.Join(benchPath, "runtime.json")
}

// Load reads the runtime config from disk
func Load(benchPath string) (*Config, error) {
	path := RuntimePath(benchPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse runtime.json: %w", err)
	}

	return &cfg, nil
}

// Save writes the runtime config to disk
func (c *Config) Save(benchPath string) error {
	path := RuntimePath(benchPath)
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal runtime config: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

// Remove deletes the runtime config file
func Remove(benchPath string) error {
	path := RuntimePath(benchPath)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// IsPortAvailable checks if a TCP port is available
func IsPortAvailable(port int) bool {
	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	listener.Close()
	return true
}

// FindAvailablePort finds an available port starting from the preferred port
// It tries up to 100 ports above the preferred one
func FindAvailablePort(preferred int) (int, error) {
	for port := preferred; port < preferred+100; port++ {
		if IsPortAvailable(port) {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available port found starting from %d", preferred)
}

// AllocatePorts finds available ports for all services
func AllocatePorts(preferred Ports) (*Ports, error) {
	var result Ports
	var err error

	result.Web, err = FindAvailablePort(preferred.Web)
	if err != nil {
		return nil, fmt.Errorf("web port: %w", err)
	}

	result.SocketIO, err = FindAvailablePort(preferred.SocketIO)
	if err != nil {
		return nil, fmt.Errorf("socketio port: %w", err)
	}

	// Redis is typically managed by devbox, so we just use the default
	result.Redis = preferred.Redis

	result.ProcessCompose, err = FindAvailablePort(preferred.ProcessCompose)
	if err != nil {
		return nil, fmt.Errorf("process-compose port: %w", err)
	}

	return &result, nil
}

// GetWebURL returns the web URL for a site using runtime ports
func (c *Config) GetWebURL(siteName string) string {
	return fmt.Sprintf("http://%s:%d", siteName, c.Ports.Web)
}

// IsRunning checks if services are still running based on the runtime config
// It checks if the web port from the config is in use
func (c *Config) IsRunning() bool {
	// Check if web port is in use (indicates services are running)
	return !IsPortAvailable(c.Ports.Web)
}

// LoadIfRunning loads the runtime config and returns it only if services appear to be running
// Returns nil, nil if no runtime config exists or services aren't running
func LoadIfRunning(benchPath string) (*Config, error) {
	cfg, err := Load(benchPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	if cfg.IsRunning() {
		return cfg, nil
	}

	// Runtime config exists but services aren't running - clean up stale config
	Remove(benchPath)
	return nil, nil
}
