package runtime

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
)

// Config holds runtime state for a running weg environment
type Config struct {
	Ports Ports `json:"ports"`
	PID   int   `json:"pid,omitempty"`
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
