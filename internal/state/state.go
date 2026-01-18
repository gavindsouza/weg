package state

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	// StateVersion is the current state file format version
	StateVersion = "1"
	// StateFileName is the name of the state file
	StateFileName = "state.json"
)

// State represents the current applied state of a weg environment
type State struct {
	Version    string               `json:"version"`
	ConfigHash string               `json:"config_hash"`
	Apps       map[string]AppState  `json:"apps"`
	Sites      map[string]SiteState `json:"sites"`
	Frappe     FrappeState          `json:"frappe"`
	Services   ServicesState        `json:"services,omitempty"`
	LastSync   time.Time            `json:"last_sync"`
}

// AppState represents the installed state of an app
type AppState struct {
	Name          string    `json:"name"`
	URL           string    `json:"url,omitempty"`
	Branch        string    `json:"branch,omitempty"`
	Commit        string    `json:"commit,omitempty"`
	Path          string    `json:"path,omitempty"`
	InstalledAt   time.Time `json:"installed_at"`
	PyprojectHash string    `json:"pyproject_hash,omitempty"` // Hash of pyproject.toml for dep change detection
}

// SiteState represents the state of a site
type SiteState struct {
	Name        string    `json:"name"`
	Apps        []string  `json:"apps"`
	CreatedAt   time.Time `json:"created_at"`
	DefaultSite bool      `json:"default,omitempty"`
}

// FrappeState represents the Frappe framework state
type FrappeState struct {
	Version  string `json:"version"`
	Database string `json:"database"`
}

// ServicesState tracks service configuration
type ServicesState struct {
	WebPort    int            `json:"web_port,omitempty"`
	SocketPort int            `json:"socket_port,omitempty"`
	Workers    map[string]int `json:"workers,omitempty"`
}

// NewState creates a new empty state
func NewState() *State {
	return &State{
		Version: StateVersion,
		Apps:    make(map[string]AppState),
		Sites:   make(map[string]SiteState),
	}
}

// Load reads the state from the .weg directory
func Load(basePath string) (*State, error) {
	statePath := getStatePath(basePath)

	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return new state if file doesn't exist
			return NewState(), nil
		}
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	// Ensure maps are initialized
	if state.Apps == nil {
		state.Apps = make(map[string]AppState)
	}
	if state.Sites == nil {
		state.Sites = make(map[string]SiteState)
	}

	return &state, nil
}

// Save writes the state to the .weg directory atomically
func (s *State) Save(basePath string) error {
	wegDir := getWegDir(basePath)
	statePath := getStatePath(basePath)

	// Ensure .weg directory exists
	if err := os.MkdirAll(wegDir, 0755); err != nil {
		return fmt.Errorf("failed to create .weg directory: %w", err)
	}

	// Marshal state to JSON
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize state: %w", err)
	}

	// Write to temp file first for atomicity
	tmpPath := statePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp state file: %w", err)
	}

	// Rename temp file to actual state file (atomic on most filesystems)
	if err := os.Rename(tmpPath, statePath); err != nil {
		os.Remove(tmpPath) // Clean up temp file
		return fmt.Errorf("failed to save state file: %w", err)
	}

	return nil
}

// UpdateConfigHash updates the config hash and last sync time
func (s *State) UpdateConfigHash(hash string) {
	s.ConfigHash = hash
	s.LastSync = time.Now()
}

// AddApp adds or updates an app in the state
func (s *State) AddApp(app AppState) {
	if app.InstalledAt.IsZero() {
		app.InstalledAt = time.Now()
	}
	s.Apps[app.Name] = app
}

// RemoveApp removes an app from the state
func (s *State) RemoveApp(name string) {
	delete(s.Apps, name)
}

// HasApp checks if an app exists in the state
func (s *State) HasApp(name string) bool {
	_, ok := s.Apps[name]
	return ok
}

// AddSite adds or updates a site in the state
func (s *State) AddSite(site SiteState) {
	if site.CreatedAt.IsZero() {
		site.CreatedAt = time.Now()
	}
	s.Sites[site.Name] = site
}

// RemoveSite removes a site from the state
func (s *State) RemoveSite(name string) {
	delete(s.Sites, name)
}

// HasSite checks if a site exists in the state
func (s *State) HasSite(name string) bool {
	_, ok := s.Sites[name]
	return ok
}

// SetDefaultSite sets a site as the default
func (s *State) SetDefaultSite(name string) {
	for siteName, site := range s.Sites {
		site.DefaultSite = (siteName == name)
		s.Sites[siteName] = site
	}
}

// GetDefaultSite returns the default site name
func (s *State) GetDefaultSite() string {
	for name, site := range s.Sites {
		if site.DefaultSite {
			return name
		}
	}
	// Return first site if no default set
	for name := range s.Sites {
		return name
	}
	return ""
}

// ComputeConfigHash computes a SHA256 hash of the config file content
func ComputeConfigHash(configPath string) (string, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("failed to read config file: %w", err)
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// ComputePyprojectHash computes a SHA256 hash of an app's pyproject.toml
// Returns empty string if pyproject.toml doesn't exist
func ComputePyprojectHash(appPath string) string {
	pyprojectPath := filepath.Join(appPath, "pyproject.toml")
	data, err := os.ReadFile(pyprojectPath)
	if err != nil {
		return ""
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// NeedsSync checks if the config has changed since last sync
func (s *State) NeedsSync(configPath string) (bool, error) {
	if s.ConfigHash == "" {
		return true, nil
	}

	currentHash, err := ComputeConfigHash(configPath)
	if err != nil {
		return false, err
	}

	return s.ConfigHash != currentHash, nil
}

// IsEmpty checks if the state has no apps or sites
func (s *State) IsEmpty() bool {
	return len(s.Apps) == 0 && len(s.Sites) == 0
}

// AppNames returns a sorted list of app names
func (s *State) AppNames() []string {
	names := make([]string, 0, len(s.Apps))
	for name := range s.Apps {
		names = append(names, name)
	}
	return names
}

// SiteNames returns a sorted list of site names
func (s *State) SiteNames() []string {
	names := make([]string, 0, len(s.Sites))
	for name := range s.Sites {
		names = append(names, name)
	}
	return names
}

// getWegDir returns the path to the .weg directory
func getWegDir(basePath string) string {
	return filepath.Join(basePath, ".weg")
}

// getStatePath returns the path to the state file
func getStatePath(basePath string) string {
	return filepath.Join(getWegDir(basePath), StateFileName)
}

// Exists checks if a state file exists at the given path
func Exists(basePath string) bool {
	statePath := getStatePath(basePath)
	_, err := os.Stat(statePath)
	return err == nil
}
