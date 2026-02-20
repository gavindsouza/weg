package cloud

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/gavindsouza/weg/internal/fsutil"
)

// CloudConfig represents cloud configuration (non-secret)
type CloudConfig struct {
	Default string                 `toml:"default,omitempty"` // Default cloud name
	Clouds  map[string]*CloudEntry `toml:"cloud,omitempty"`
}

// CloudEntry represents a single cloud instance configuration
type CloudEntry struct {
	URL  string `toml:"url,omitempty"`
	Team string `toml:"team,omitempty"`
}

// CloudCredentials represents cloud credentials (secret)
type CloudCredentials struct {
	Clouds map[string]*CloudAuth `toml:"cloud,omitempty"`
}

// CloudAuth represents authentication for a cloud instance
type CloudAuth struct {
	Token string `toml:"token,omitempty"` // api_key:api_secret
}

// GlobalConfigDir returns the global config directory (~/.config/weg or $XDG_CONFIG_HOME/weg)
func GlobalConfigDir() (string, error) {
	// Check XDG_CONFIG_HOME first (Linux/Unix convention)
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "weg"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".config", "weg"), nil
}

// ConfigPaths returns the paths to check for config files
// Returns (globalConfig, localConfig, globalCreds, localCreds)
func ConfigPaths() (string, string, string, string, error) {
	globalDir, err := GlobalConfigDir()
	if err != nil {
		return "", "", "", "", err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", "", "", "", err
	}

	globalConfig := filepath.Join(globalDir, "config.toml")
	globalCreds := filepath.Join(globalDir, "credentials.toml")
	localConfig := filepath.Join(cwd, ".weg", "config.toml")
	localCreds := filepath.Join(cwd, ".weg", "credentials.toml")

	return globalConfig, localConfig, globalCreds, localCreds, nil
}

// LoadConfig loads cloud configuration with local overriding global
func LoadConfig() (*CloudConfig, error) {
	globalPath, localPath, _, _, err := ConfigPaths()
	if err != nil {
		return nil, err
	}

	config := &CloudConfig{
		Clouds: make(map[string]*CloudEntry),
	}

	// Load global first
	if data, err := os.ReadFile(globalPath); err == nil {
		if _, err := toml.Decode(string(data), config); err != nil {
			return nil, fmt.Errorf("invalid global config: %w", err)
		}
	}

	// Load local (overrides global)
	if data, err := os.ReadFile(localPath); err == nil {
		localConfig := &CloudConfig{Clouds: make(map[string]*CloudEntry)}
		if _, err := toml.Decode(string(data), localConfig); err != nil {
			return nil, fmt.Errorf("invalid local config: %w", err)
		}
		// Merge: local overrides global
		if localConfig.Default != "" {
			config.Default = localConfig.Default
		}
		for name, entry := range localConfig.Clouds {
			config.Clouds[name] = entry
		}
	}

	return config, nil
}

// LoadCredentials loads cloud credentials with local overriding global
func LoadCredentials() (*CloudCredentials, error) {
	_, _, globalPath, localPath, err := ConfigPaths()
	if err != nil {
		return nil, err
	}

	creds := &CloudCredentials{
		Clouds: make(map[string]*CloudAuth),
	}

	// Load global first
	if data, err := os.ReadFile(globalPath); err == nil {
		if _, err := toml.Decode(string(data), creds); err != nil {
			return nil, fmt.Errorf("invalid global credentials: %w", err)
		}
	}

	// Load local (overrides global)
	if data, err := os.ReadFile(localPath); err == nil {
		localCreds := &CloudCredentials{Clouds: make(map[string]*CloudAuth)}
		if _, err := toml.Decode(string(data), localCreds); err != nil {
			return nil, fmt.Errorf("invalid local credentials: %w", err)
		}
		// Merge: local overrides global
		for name, auth := range localCreds.Clouds {
			creds.Clouds[name] = auth
		}
	}

	return creds, nil
}

// SaveConfig saves cloud configuration
func SaveConfig(config *CloudConfig, global bool) error {
	globalPath, localPath, _, _, err := ConfigPaths()
	if err != nil {
		return err
	}

	path := localPath
	if global {
		path = globalPath
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	// Encode to TOML
	data, err := tomlMarshal(config)
	if err != nil {
		return err
	}

	return fsutil.AtomicWrite(path, data, 0644)
}

// SaveCredentials saves cloud credentials with restricted permissions
func SaveCredentials(creds *CloudCredentials, global bool) error {
	_, _, globalPath, localPath, err := ConfigPaths()
	if err != nil {
		return err
	}

	path := localPath
	if global {
		path = globalPath
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	// Encode to TOML
	data, err := tomlMarshal(creds)
	if err != nil {
		return err
	}

	// Write with restricted permissions (0600 = owner read/write only)
	return fsutil.AtomicWrite(path, data, 0600)
}

// GetCloudClient returns a configured client for the specified cloud (or default)
func GetCloudClient(cloudName string) (*Client, error) {
	config, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	creds, err := LoadCredentials()
	if err != nil {
		return nil, err
	}

	// Use default if not specified
	if cloudName == "" {
		cloudName = config.Default
	}
	if cloudName == "" {
		cloudName = "frappe" // fallback default
	}

	// Get cloud entry
	entry := config.Clouds[cloudName]
	if entry == nil {
		entry = &CloudEntry{}
	}

	// Get credentials
	auth := creds.Clouds[cloudName]
	if auth == nil || auth.Token == "" {
		return nil, fmt.Errorf("not logged in to cloud '%s': run 'weg cloud login'", cloudName)
	}

	// Build URL
	url := entry.URL
	if url == "" {
		url = DefaultCloudAPI
	}

	client := NewClientWithURL(auth.Token, url)
	if entry.Team != "" {
		client.Team = entry.Team
	}

	return client, nil
}

// tomlMarshal encodes a value to TOML bytes
func tomlMarshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	encoder := toml.NewEncoder(&buf)
	if err := encoder.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
