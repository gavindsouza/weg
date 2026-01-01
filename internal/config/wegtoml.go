package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// BenchConfig represents a weg.toml configuration file
type BenchConfig struct {
	Bench    BenchSettings          `toml:"bench"`
	Frappe   FrappeSettings         `toml:"frappe"`
	Apps     map[string]AppSettings `toml:"apps"`
	Sites    []SiteConfig           `toml:"sites"`
	Services ServicesConfig         `toml:"services"`
}

// BenchSettings contains bench-level configuration
type BenchSettings struct {
	Name string `toml:"name"`
	Path string `toml:"path,omitempty"` // Optional, defaults to current directory
}

// FrappeSettings contains Frappe framework configuration
type FrappeSettings struct {
	Version  string `toml:"version"`  // e.g., "15" or "version-15"
	Database string `toml:"database"` // mariadb, postgres, sqlite
}

// AppSettings represents configuration for a single app
type AppSettings struct {
	URL      string `toml:"url,omitempty"`
	Branch   string `toml:"branch,omitempty"`
	Path     string `toml:"path,omitempty"` // For local development
	Excluded bool   `toml:"excluded,omitempty"`
}

// SiteConfig represents a site configuration
type SiteConfig struct {
	Name         string   `toml:"name"`
	Apps         []string `toml:"apps,omitempty"`          // Apps to install on site
	AdminPass    string   `toml:"admin_password,omitempty"`
	DefaultSite  bool     `toml:"default,omitempty"`
	Domains      []string `toml:"domains,omitempty"`
}

// ServicesConfig contains service configuration overrides
type ServicesConfig struct {
	Redis    RedisConfig    `toml:"redis,omitempty"`
	Database DatabaseConfig `toml:"database,omitempty"`
	Web      WebConfig      `toml:"web,omitempty"`
}

// RedisConfig contains Redis service configuration
type RedisConfig struct {
	CachePort int `toml:"cache_port,omitempty"`
	QueuePort int `toml:"queue_port,omitempty"`
}

// DatabaseConfig contains database service configuration
type DatabaseConfig struct {
	Host     string `toml:"host,omitempty"`
	Port     int    `toml:"port,omitempty"`
	RootPass string `toml:"root_password,omitempty"`
}

// WebConfig contains web server configuration
type WebConfig struct {
	Port       int  `toml:"port,omitempty"`
	SocketPort int  `toml:"socket_port,omitempty"`
	DevMode    bool `toml:"dev_mode,omitempty"`
}

// ParseWegToml reads and parses a weg.toml file
func ParseWegToml(path string) (*BenchConfig, error) {
	wegPath := filepath.Join(path, "weg.toml")

	data, err := os.ReadFile(wegPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("weg.toml not found at %s", path)
		}
		return nil, fmt.Errorf("failed to read weg.toml: %w", err)
	}

	var config BenchConfig
	if err := toml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse weg.toml: %w", err)
	}

	// Apply defaults
	applyBenchDefaults(&config, path)

	return &config, nil
}

// applyBenchDefaults applies sensible defaults to the bench configuration
func applyBenchDefaults(config *BenchConfig, path string) {
	if config.Bench.Name == "" {
		config.Bench.Name = filepath.Base(path)
	}
	if config.Bench.Path == "" {
		config.Bench.Path = path
	}
	if config.Frappe.Version == "" {
		config.Frappe.Version = "15"
	}
	if config.Frappe.Database == "" {
		config.Frappe.Database = "mariadb"
	}

	// Ensure Frappe is in apps if not explicitly defined
	if config.Apps == nil {
		config.Apps = make(map[string]AppSettings)
	}
	if _, ok := config.Apps["frappe"]; !ok {
		config.Apps["frappe"] = AppSettings{
			URL:    "https://github.com/frappe/frappe",
			Branch: resolveVersionBranch(config.Frappe.Version),
		}
	}

	// Apply service defaults
	if config.Services.Redis.CachePort == 0 {
		config.Services.Redis.CachePort = 13000
	}
	if config.Services.Redis.QueuePort == 0 {
		config.Services.Redis.QueuePort = 11000
	}
	if config.Services.Web.Port == 0 {
		config.Services.Web.Port = 8000
	}
	if config.Services.Web.SocketPort == 0 {
		config.Services.Web.SocketPort = 9000
	}
}

// resolveVersionBranch converts a version number to a branch name
func resolveVersionBranch(version string) string {
	switch version {
	case "14":
		return "version-14"
	case "15":
		return "version-15"
	case "16":
		return "version-16"
	case "develop":
		return "develop"
	default:
		// If it already looks like a branch, use as-is
		return version
	}
}

// HasWegToml checks if a weg.toml file exists at the given path
func HasWegToml(path string) bool {
	wegPath := filepath.Join(path, "weg.toml")
	_, err := os.Stat(wegPath)
	return err == nil
}

// ValidateBenchConfig validates the bench configuration
func ValidateBenchConfig(config *BenchConfig) error {
	// Validate Frappe version
	validVersions := map[string]bool{
		"14": true, "15": true, "16": true, "develop": true,
		"version-14": true, "version-15": true, "version-16": true,
	}
	if !validVersions[config.Frappe.Version] {
		return fmt.Errorf("invalid Frappe version %q", config.Frappe.Version)
	}

	// Validate database
	validDBs := map[string]bool{"mariadb": true, "postgres": true, "sqlite": true}
	if !validDBs[config.Frappe.Database] {
		return fmt.Errorf("invalid database %q: must be one of mariadb, postgres, sqlite", config.Frappe.Database)
	}

	// SQLite only supported in v16+
	if config.Frappe.Database == "sqlite" {
		v := config.Frappe.Version
		if v != "16" && v != "version-16" && v != "develop" {
			return fmt.Errorf("sqlite database requires Frappe version 16 or develop")
		}
	}

	// Validate apps
	for name, app := range config.Apps {
		if app.URL == "" && app.Path == "" && name != "frappe" {
			return fmt.Errorf("app %q must have either url or path specified", name)
		}
	}

	// Validate sites
	defaultCount := 0
	for _, site := range config.Sites {
		if site.Name == "" {
			return fmt.Errorf("site must have a name")
		}
		if site.DefaultSite {
			defaultCount++
		}
	}
	if defaultCount > 1 {
		return fmt.Errorf("only one site can be marked as default")
	}

	return nil
}

// GetApp returns an app configuration by name
func (c *BenchConfig) GetApp(name string) (AppSettings, bool) {
	app, ok := c.Apps[name]
	return app, ok
}

// GetDefaultSite returns the default site configuration
func (c *BenchConfig) GetDefaultSite() *SiteConfig {
	for i := range c.Sites {
		if c.Sites[i].DefaultSite {
			return &c.Sites[i]
		}
	}
	if len(c.Sites) > 0 {
		return &c.Sites[0]
	}
	return nil
}

// AppNames returns a list of all app names
func (c *BenchConfig) AppNames() []string {
	names := make([]string, 0, len(c.Apps))
	for name := range c.Apps {
		names = append(names, name)
	}
	return names
}

// EnabledApps returns only non-excluded apps
func (c *BenchConfig) EnabledApps() map[string]AppSettings {
	enabled := make(map[string]AppSettings)
	for name, app := range c.Apps {
		if !app.Excluded {
			enabled[name] = app
		}
	}
	return enabled
}
