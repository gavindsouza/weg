package tools

import (
	"fmt"
	"regexp"
)

// Dependency represents a system dependency with name and version
type Dependency struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// FrappeVersion represents a Frappe version and its dependencies
type FrappeVersion struct {
	Version           string       `json:"version"`
	VersionPattern    string       `json:"versionPattern"`
	Dependencies      []Dependency `json:"dependencies"`
	SupportedDBs      []string     `json:"supported_dbs"`
	PythonVersion     string       `json:"python_version"`
	NodeVersion       string       `json:"node_version"`
	PackageManager    string       `json:"package_manager"` // yarn, pnpm, bun
}

// Frappe holds all version configurations
type Frappe struct {
	Versions []FrappeVersion `json:"versions"`
}

var frappe = Frappe{
	Versions: []FrappeVersion{
		{
			Version:        "14.x.x",
			VersionPattern: `^(v?14\.|version-14(-.*)?)`,
			SupportedDBs:   []string{"mariadb", "postgres"},
			PythonVersion:  "3.10",
			NodeVersion:    "16",
			PackageManager: "yarn",
			Dependencies: []Dependency{
				{Name: "python", Version: "3.10"},
				{Name: "nodejs", Version: "16"},
				{Name: "redis", Version: "6.2"},
				{Name: "mariadb", Version: "10.6"},
				{Name: "wkhtmltopdf", Version: "0.12.6"},
				{Name: "yarn", Version: "1.22"},
			},
		},
		{
			Version:        "15.x.x",
			VersionPattern: `^(v?15\.|version-15(-.*)?)`,
			SupportedDBs:   []string{"mariadb", "postgres"},
			PythonVersion:  "3.11",
			NodeVersion:    "18",
			PackageManager: "yarn",
			Dependencies: []Dependency{
				{Name: "python", Version: "3.11"},
				{Name: "nodejs", Version: "18"},
				{Name: "redis", Version: "6.2"},
				{Name: "mariadb", Version: "10.6"},
				{Name: "wkhtmltopdf", Version: "0.12.6"},
				{Name: "yarn", Version: "1.22"},
			},
		},
		{
			Version:        "16.x.x",
			VersionPattern: `^(v?16\.|version-16(-.*)?)`,
			SupportedDBs:   []string{"mariadb", "postgres", "sqlite"},
			PythonVersion:  "3.12",
			NodeVersion:    "20",
			PackageManager: "pnpm",
			Dependencies: []Dependency{
				{Name: "python", Version: "3.12"},
				{Name: "nodejs", Version: "20"},
				{Name: "redis", Version: "7"},
				{Name: "mariadb", Version: "10.11"},
				{Name: "wkhtmltopdf", Version: "0.12.6"},
				{Name: "pnpm", Version: "9"},
			},
		},
		{
			Version:        "develop",
			VersionPattern: `^develop$`,
			SupportedDBs:   []string{"mariadb", "postgres", "sqlite"},
			PythonVersion:  "3.13",
			NodeVersion:    "22",
			PackageManager: "pnpm",
			Dependencies: []Dependency{
				{Name: "python", Version: "3.13"},
				{Name: "nodejs", Version: "22"},
				{Name: "redis", Version: "7"},
				{Name: "mariadb", Version: "10.11"},
				{Name: "wkhtmltopdf", Version: "0.12.6"},
				{Name: "pnpm", Version: "9"},
			},
		},
	},
}

// GetDependencies returns dependencies for a given Frappe version
func GetDependencies(version string) ([]Dependency, error) {
	for _, v := range frappe.Versions {
		matched, _ := regexp.MatchString(v.VersionPattern, version)
		if matched {
			return v.Dependencies, nil
		}
	}
	return nil, fmt.Errorf("version %s not found", version)
}

// GetFrappeVersion returns the full FrappeVersion config for a version string
func GetFrappeVersion(version string) (*FrappeVersion, error) {
	for i, v := range frappe.Versions {
		matched, _ := regexp.MatchString(v.VersionPattern, version)
		if matched {
			return &frappe.Versions[i], nil
		}
	}
	return nil, fmt.Errorf("version %s not found", version)
}

// GetDependenciesForDB returns dependencies with the specified database
func GetDependenciesForDB(version, database string) ([]Dependency, error) {
	fv, err := GetFrappeVersion(version)
	if err != nil {
		return nil, err
	}

	// Check if database is supported
	supported := false
	for _, db := range fv.SupportedDBs {
		if db == database {
			supported = true
			break
		}
	}
	if !supported {
		return nil, fmt.Errorf("database %s not supported for Frappe %s (supported: %v)", database, version, fv.SupportedDBs)
	}

	// Copy dependencies and replace mariadb if needed
	deps := make([]Dependency, 0, len(fv.Dependencies))
	for _, dep := range fv.Dependencies {
		if dep.Name == "mariadb" && database != "mariadb" {
			switch database {
			case "postgres":
				deps = append(deps, Dependency{Name: "postgresql", Version: "15"})
			case "sqlite":
				// SQLite is built into Python, no extra dep needed
				continue
			}
		} else {
			deps = append(deps, dep)
		}
	}

	return deps, nil
}

// IsDatabaseSupported checks if a database is supported for a Frappe version
func IsDatabaseSupported(version, database string) bool {
	// Normalize version first
	normalizedVersion := NormalizeFrappeVersion(version)
	fv, err := GetFrappeVersion(normalizedVersion)
	if err != nil {
		// Try with original version in case it's already normalized
		fv, err = GetFrappeVersion(version)
		if err != nil {
			return false
		}
	}
	for _, db := range fv.SupportedDBs {
		if db == database {
			return true
		}
	}
	return false
}

// GetSupportedVersions returns all supported Frappe version strings
func GetSupportedVersions() []string {
	return []string{"14", "15", "16", "develop"}
}

// GetSupportedDatabases returns all supported databases
func GetSupportedDatabases() []string {
	return []string{"mariadb", "postgres", "sqlite"}
}

// NormalizeFrappeVersion converts version shortcuts to full version patterns
func NormalizeFrappeVersion(version string) string {
	switch version {
	case "14":
		return "version-14"
	case "15":
		return "version-15"
	case "16":
		return "version-16"
	default:
		return version
	}
}
