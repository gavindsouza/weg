/*
Copyright © 2025 Gavin <me@gavv.in>

Remote site configuration handling.
Manages .weg/site.toml and .weg/credentials.toml files.
*/
package remote

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// SiteConfig represents the .weg/site.toml configuration
type SiteConfig struct {
	Site     SiteInfo              `toml:"site"`
	Modules  map[string]ModuleInfo `toml:"modules"`
	Sync     SyncSettings          `toml:"sync"`
	Versions map[string]VersionInfo `toml:"versions"`
}

// SiteInfo contains basic site information
type SiteInfo struct {
	URL       string    `toml:"url"`
	Name      string    `toml:"name"`
	ClonedAt  time.Time `toml:"cloned_at"`
	Auth      AuthInfo  `toml:"auth"`
	Frappe    FrappeInfo `toml:"frappe"`
	Apps      map[string]AppInfo `toml:"apps"`
}

// AuthInfo contains authentication method (credentials stored separately)
type AuthInfo struct {
	Method string `toml:"method"` // "api_key", "password", "oauth"
}

// FrappeInfo contains Frappe version information
type FrappeInfo struct {
	Version string `toml:"version"`
}

// AppInfo contains installed app information
type AppInfo struct {
	Version string `toml:"version"`
}

// ModuleInfo contains module sync configuration
type ModuleInfo struct {
	App  string `toml:"app"`
	Sync bool   `toml:"sync"`
}

// SyncSettings contains sync configuration
type SyncSettings struct {
	LastSync time.Time            `toml:"last_sync"`
	Entities EntitySyncSettings   `toml:"entities"`
	Exclude  ExcludeSettings      `toml:"exclude"`
}

// EntitySyncSettings controls which entity types to sync
type EntitySyncSettings struct {
	DocType        bool `toml:"doctype"`
	CustomField    bool `toml:"custom_field"`
	PropertySetter bool `toml:"property_setter"`
	ClientScript   bool `toml:"client_script"`
	ServerScript   bool `toml:"server_script"`
	Report         bool `toml:"report"`
	PrintFormat    bool `toml:"print_format"`
	Workspace      bool `toml:"workspace"`
	Notification   bool `toml:"notification"`
	Workflow       bool `toml:"workflow"`
	LetterHead     bool `toml:"letter_head"`
	WebTemplate    bool `toml:"web_template"`
	NumberCard     bool `toml:"number_card"`
	Dashboard      bool `toml:"dashboard"`
	DashboardChart bool `toml:"dashboard_chart"`
}

// ExcludeSettings contains exclusion patterns
type ExcludeSettings struct {
	Patterns []string `toml:"patterns"`
}

// VersionInfo tracks sync state for a file
type VersionInfo struct {
	Version  string    `toml:"version"`
	Modified time.Time `toml:"modified"`
}

// Credentials represents the .weg/credentials.toml file (gitignored)
type Credentials struct {
	Auth CredentialAuth `toml:"auth"`
}

// CredentialAuth contains actual credentials
type CredentialAuth struct {
	APIKey    string `toml:"api_key"`
	APISecret string `toml:"api_secret"`
	Username  string `toml:"username"`
	Password  string `toml:"password"`
}

// GlobalCredentials represents ~/.config/weg/credentials.toml
// Sites are keyed by hostname
type GlobalCredentials struct {
	Sites map[string]*CredentialAuth `toml:"site"`
}

// NewSiteConfig creates a new SiteConfig with sensible defaults
func NewSiteConfig(url, name string) *SiteConfig {
	return &SiteConfig{
		Site: SiteInfo{
			URL:      url,
			Name:     name,
			ClonedAt: time.Now(),
			Auth: AuthInfo{
				Method: "api_key",
			},
			Apps: make(map[string]AppInfo),
		},
		Modules: map[string]ModuleInfo{
			"Custom": {App: "_site", Sync: true},
			"_":      {App: "_site", Sync: true}, // Catch-all
		},
		Sync: SyncSettings{
			Entities: EntitySyncSettings{
				DocType:        true,
				CustomField:    true,
				PropertySetter: true,
				ClientScript:   true,
				ServerScript:   true,
				Report:         true,
				PrintFormat:    true,
				Workspace:      false, // Off by default
				Notification:   false, // Off by default
				Workflow:       true,
				LetterHead:     true,
				WebTemplate:    false,
				NumberCard:     false,
				Dashboard:      false,
				DashboardChart: false,
			},
			Exclude: ExcludeSettings{
				Patterns: []string{},
			},
		},
		Versions: make(map[string]VersionInfo),
	}
}

// LoadSiteConfig loads configuration from .weg/site.toml
func LoadSiteConfig(dir string) (*SiteConfig, error) {
	configPath := filepath.Join(dir, ".weg", "site.toml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read site config: %w", err)
	}

	var config SiteConfig
	if err := toml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse site config: %w", err)
	}

	return &config, nil
}

// Save writes the configuration to .weg/site.toml
func (c *SiteConfig) Save(dir string) error {
	wegDir := filepath.Join(dir, ".weg")
	if err := os.MkdirAll(wegDir, 0755); err != nil {
		return fmt.Errorf("failed to create .weg directory: %w", err)
	}

	configPath := filepath.Join(wegDir, "site.toml")
	f, err := os.Create(configPath)
	if err != nil {
		return fmt.Errorf("failed to create site config: %w", err)
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(c); err != nil {
		return fmt.Errorf("failed to write site config: %w", err)
	}

	return nil
}

// GlobalConfigDir returns the global config directory (~/.config/weg or $XDG_CONFIG_HOME/weg)
func GlobalConfigDir() (string, error) {
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "weg"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".config", "weg"), nil
}

// LoadCredentials loads credentials with hierarchy: env → local → global
func LoadCredentials(dir string) (*Credentials, error) {
	// Load site config to get the URL for global lookup
	config, _ := LoadSiteConfig(dir)
	var siteHost string
	if config != nil {
		siteHost = extractHost(config.Site.URL)
	}

	return LoadCredentialsForSite(dir, siteHost)
}

// LoadCredentialsForSite loads credentials with hierarchy: env → local → global (by hostname)
func LoadCredentialsForSite(dir string, siteHost string) (*Credentials, error) {
	// 1. Check environment variables (highest priority)
	apiKey := os.Getenv("WEG_API_KEY")
	apiSecret := os.Getenv("WEG_API_SECRET")
	if apiKey != "" && apiSecret != "" {
		return &Credentials{
			Auth: CredentialAuth{
				APIKey:    apiKey,
				APISecret: apiSecret,
			},
		}, nil
	}

	// 2. Check local credentials file
	credPath := filepath.Join(dir, ".weg", "credentials.toml")
	if data, err := os.ReadFile(credPath); err == nil {
		var creds Credentials
		if err := toml.Unmarshal(data, &creds); err == nil {
			if creds.Auth.APIKey != "" && creds.Auth.APISecret != "" {
				return &creds, nil
			}
		}
	}

	// 3. Check global credentials file (keyed by site hostname)
	if siteHost != "" {
		globalDir, err := GlobalConfigDir()
		if err == nil {
			globalCredPath := filepath.Join(globalDir, "credentials.toml")
			if data, err := os.ReadFile(globalCredPath); err == nil {
				var globalCreds GlobalCredentials
				if err := toml.Unmarshal(data, &globalCreds); err == nil {
					if auth := globalCreds.Sites[siteHost]; auth != nil {
						if auth.APIKey != "" && auth.APISecret != "" {
							return &Credentials{Auth: *auth}, nil
						}
					}
				}
			}
		}
	}

	return nil, fmt.Errorf("no credentials found (checked: env, local .weg/credentials.toml, global ~/.config/weg/credentials.toml)")
}

// extractHost extracts hostname from a URL
func extractHost(urlStr string) string {
	// Simple extraction - handle common cases
	urlStr = strings.TrimPrefix(urlStr, "https://")
	urlStr = strings.TrimPrefix(urlStr, "http://")
	if idx := strings.Index(urlStr, "/"); idx != -1 {
		urlStr = urlStr[:idx]
	}
	if idx := strings.Index(urlStr, ":"); idx != -1 {
		urlStr = urlStr[:idx]
	}
	return urlStr
}

// SaveCredentials writes credentials to .weg/credentials.toml
func (c *Credentials) Save(dir string) error {
	wegDir := filepath.Join(dir, ".weg")
	if err := os.MkdirAll(wegDir, 0755); err != nil {
		return fmt.Errorf("failed to create .weg directory: %w", err)
	}

	credPath := filepath.Join(wegDir, "credentials.toml")
	f, err := os.OpenFile(credPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600) // Restricted permissions
	if err != nil {
		return fmt.Errorf("failed to create credentials file: %w", err)
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(c); err != nil {
		return fmt.Errorf("failed to write credentials: %w", err)
	}

	return nil
}

// LoadGlobalCredentials loads the global credentials file
func LoadGlobalCredentials() (*GlobalCredentials, error) {
	globalDir, err := GlobalConfigDir()
	if err != nil {
		return nil, err
	}

	globalCredPath := filepath.Join(globalDir, "credentials.toml")
	data, err := os.ReadFile(globalCredPath)
	if err != nil {
		// Return empty if file doesn't exist
		if os.IsNotExist(err) {
			return &GlobalCredentials{Sites: make(map[string]*CredentialAuth)}, nil
		}
		return nil, err
	}

	var creds GlobalCredentials
	if err := toml.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("failed to parse global credentials: %w", err)
	}

	if creds.Sites == nil {
		creds.Sites = make(map[string]*CredentialAuth)
	}

	return &creds, nil
}

// SaveGlobalCredentials saves credentials to the global config
func SaveGlobalCredentials(siteHost string, auth *CredentialAuth) error {
	globalDir, err := GlobalConfigDir()
	if err != nil {
		return err
	}

	// Ensure directory exists
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Load existing credentials
	creds, err := LoadGlobalCredentials()
	if err != nil {
		creds = &GlobalCredentials{Sites: make(map[string]*CredentialAuth)}
	}

	// Add/update this site's credentials
	creds.Sites[siteHost] = auth

	// Write with restricted permissions
	globalCredPath := filepath.Join(globalDir, "credentials.toml")
	f, err := os.OpenFile(globalCredPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create credentials file: %w", err)
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(creds); err != nil {
		return fmt.Errorf("failed to write credentials: %w", err)
	}

	return nil
}

// HasGlobalCredentials checks if credentials exist globally for a site
func HasGlobalCredentials(siteHost string) bool {
	creds, err := LoadGlobalCredentials()
	if err != nil {
		return false
	}
	auth := creds.Sites[siteHost]
	return auth != nil && auth.APIKey != "" && auth.APISecret != ""
}

// RemoveGlobalCredentials removes credentials for a site from global config
func RemoveGlobalCredentials(siteHost string) error {
	creds, err := LoadGlobalCredentials()
	if err != nil {
		return err
	}

	if _, exists := creds.Sites[siteHost]; !exists {
		return nil // Already doesn't exist
	}

	delete(creds.Sites, siteHost)

	// Write updated credentials
	globalDir, err := GlobalConfigDir()
	if err != nil {
		return err
	}

	globalCredPath := filepath.Join(globalDir, "credentials.toml")
	f, err := os.OpenFile(globalCredPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to open credentials file: %w", err)
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(creds); err != nil {
		return fmt.Errorf("failed to write credentials: %w", err)
	}

	return nil
}

// IsRemoteSite checks if a directory is a remote site clone
func IsRemoteSite(dir string) bool {
	configPath := filepath.Join(dir, ".weg", "site.toml")
	_, err := os.Stat(configPath)
	return err == nil
}

// EnsureGitignore ensures sensitive/local files are in .gitignore
func EnsureGitignore(dir string) error {
	gitignorePath := filepath.Join(dir, ".weg", ".gitignore")
	content := `# Credentials - NEVER commit
credentials.toml

# Local state - machine-specific
last_push_commit
workspace_state.json
`
	return os.WriteFile(gitignorePath, []byte(content), 0644)
}
