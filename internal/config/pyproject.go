package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// AppConfig represents the [tool.weg] section in pyproject.toml
type AppConfig struct {
	Compatibility CompatibilityConfig `toml:"compatibility"`
	Dev           DevConfig           `toml:"dev"`
	Dependencies  DependenciesConfig  `toml:"dependencies"`
	Services      AppServicesConfig   `toml:"services"`
}

// AppServicesConfig defines additional services an app needs
type AppServicesConfig struct {
	Packages  []string                    `toml:"packages"`  // Extra devbox packages (e.g., ["tor@latest"])
	Processes map[string]AppProcessConfig `toml:"processes"` // Extra processes to run
}

// AppProcessConfig defines a process for process-compose
type AppProcessConfig struct {
	Command     string            `toml:"command"`
	WorkingDir  string            `toml:"working_dir,omitempty"`
	Environment map[string]string `toml:"environment,omitempty"`
	DependsOn   []string          `toml:"depends_on,omitempty"`
}

// CompatibilityConfig defines which Frappe versions and databases the app supports
type CompatibilityConfig struct {
	Frappe    []string `toml:"frappe"`    // e.g., ["14", "15", "16"]
	Databases []string `toml:"databases"` // e.g., ["mariadb", "postgres", "sqlite"]
}

// DevConfig defines development environment settings
type DevConfig struct {
	Frappe   string `toml:"frappe"`   // e.g., "15"
	Database string `toml:"database"` // e.g., "mariadb"
}

// DependenciesConfig defines app dependencies
type DependenciesConfig struct {
	Apps []AppDependency `toml:"apps"`
}

// AppDependency represents a dependency on another Frappe app
type AppDependency struct {
	Name   string `toml:"name"`
	URL    string `toml:"url,omitempty"`
	Branch string `toml:"branch,omitempty"`
}

// pyprojectFile represents the full pyproject.toml structure
type pyprojectFile struct {
	Tool toolSection `toml:"tool"`
}

type toolSection struct {
	Weg AppConfig `toml:"weg"`
}

// ParsePyproject reads and parses the [tool.weg] section from pyproject.toml
func ParsePyproject(path string) (*AppConfig, error) {
	pyprojectPath := filepath.Join(path, "pyproject.toml")

	data, err := os.ReadFile(pyprojectPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("pyproject.toml not found at %s", path)
		}
		return nil, fmt.Errorf("failed to read pyproject.toml: %w", err)
	}

	var pf pyprojectFile
	if err := toml.Unmarshal(data, &pf); err != nil {
		return nil, fmt.Errorf("failed to parse pyproject.toml: %w", err)
	}

	config := &pf.Tool.Weg

	// Apply defaults
	if config.Compatibility.Frappe == nil {
		config.Compatibility.Frappe = []string{"15"}
	}
	if config.Compatibility.Databases == nil {
		config.Compatibility.Databases = []string{"mariadb"}
	}
	if config.Dev.Frappe == "" {
		// Default to first compatible version
		config.Dev.Frappe = config.Compatibility.Frappe[0]
	}
	if config.Dev.Database == "" {
		// Default to first compatible database
		config.Dev.Database = config.Compatibility.Databases[0]
	}

	return config, nil
}

// HasWegSection checks if pyproject.toml contains a [tool.weg] section
func HasWegSection(path string) bool {
	pyprojectPath := filepath.Join(path, "pyproject.toml")

	data, err := os.ReadFile(pyprojectPath)
	if err != nil {
		return false
	}

	var pf pyprojectFile
	if err := toml.Unmarshal(data, &pf); err != nil {
		return false
	}

	// Check if any weg config exists by checking if defaults would be applied
	return pf.Tool.Weg.Compatibility.Frappe != nil ||
		pf.Tool.Weg.Dev.Frappe != "" ||
		pf.Tool.Weg.Dependencies.Apps != nil
}

// CollectAppServices reads [tool.weg.services] from all apps in appsDir
// Returns merged packages and processes from all apps
func CollectAppServices(appsDir string) (packages []string, processes map[string]AppProcessConfig, err error) {
	processes = make(map[string]AppProcessConfig)
	seenPackages := make(map[string]bool)

	entries, err := os.ReadDir(appsDir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read apps directory: %w", err)
	}

	for _, entry := range entries {
		// Follow symlinks for IsDir check
		appPath := filepath.Join(appsDir, entry.Name())
		info, err := os.Stat(appPath)
		if err != nil || !info.IsDir() {
			continue
		}

		pyprojectPath := filepath.Join(appPath, "pyproject.toml")

		data, err := os.ReadFile(pyprojectPath)
		if err != nil {
			continue // Skip apps without pyproject.toml
		}

		var pf pyprojectFile
		if err := toml.Unmarshal(data, &pf); err != nil {
			continue // Skip invalid pyproject.toml
		}

		// Collect packages (deduplicated)
		for _, pkg := range pf.Tool.Weg.Services.Packages {
			if !seenPackages[pkg] {
				seenPackages[pkg] = true
				packages = append(packages, pkg)
			}
		}

		// Collect processes (app name prefixed to avoid conflicts)
		for name, proc := range pf.Tool.Weg.Services.Processes {
			processes[name] = proc
		}
	}

	return packages, processes, nil
}

// ValidateAppConfig validates the app configuration
func ValidateAppConfig(config *AppConfig) error {
	// Validate Frappe versions
	validVersions := map[string]bool{"14": true, "15": true, "16": true}
	for _, v := range config.Compatibility.Frappe {
		if !validVersions[v] {
			return fmt.Errorf("invalid Frappe version %q: must be one of 14, 15, 16", v)
		}
	}

	// Validate databases
	validDBs := map[string]bool{"mariadb": true, "postgres": true, "sqlite": true}
	for _, db := range config.Compatibility.Databases {
		if !validDBs[db] {
			return fmt.Errorf("invalid database %q: must be one of mariadb, postgres, sqlite", db)
		}
	}

	// Validate dev settings against compatibility
	if config.Dev.Frappe != "" {
		found := false
		for _, v := range config.Compatibility.Frappe {
			if v == config.Dev.Frappe {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("dev.frappe %q is not in compatibility.frappe list", config.Dev.Frappe)
		}
	}

	if config.Dev.Database != "" {
		found := false
		for _, db := range config.Compatibility.Databases {
			if db == config.Dev.Database {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("dev.database %q is not in compatibility.databases list", config.Dev.Database)
		}
	}

	// SQLite only supported in v16+
	for _, db := range config.Compatibility.Databases {
		if db == "sqlite" {
			hasSqliteCompatible := false
			for _, v := range config.Compatibility.Frappe {
				if v == "16" {
					hasSqliteCompatible = true
					break
				}
			}
			if !hasSqliteCompatible {
				return fmt.Errorf("sqlite database requires Frappe version 16 in compatibility.frappe")
			}
		}
	}

	return nil
}
