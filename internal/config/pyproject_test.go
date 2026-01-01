package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParsePyproject(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Write test pyproject.toml
	content := `[project]
name = "test-app"
version = "0.0.1"

[tool.weg]
[tool.weg.compatibility]
frappe = ["14", "15", "16"]
databases = ["mariadb", "postgres"]

[tool.weg.dev]
frappe = "15"
database = "mariadb"

[[tool.weg.dependencies.apps]]
name = "payments"
url = "https://github.com/frappe/payments"
branch = "version-15"
`
	pyprojectPath := filepath.Join(tmpDir, "pyproject.toml")
	if err := os.WriteFile(pyprojectPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Parse
	config, err := ParsePyproject(tmpDir)
	if err != nil {
		t.Fatalf("ParsePyproject failed: %v", err)
	}

	// Verify compatibility
	if len(config.Compatibility.Frappe) != 3 {
		t.Errorf("expected 3 Frappe versions, got %d", len(config.Compatibility.Frappe))
	}
	if config.Compatibility.Frappe[0] != "14" {
		t.Errorf("expected first version to be 14, got %s", config.Compatibility.Frappe[0])
	}

	if len(config.Compatibility.Databases) != 2 {
		t.Errorf("expected 2 databases, got %d", len(config.Compatibility.Databases))
	}

	// Verify dev settings
	if config.Dev.Frappe != "15" {
		t.Errorf("expected dev frappe 15, got %s", config.Dev.Frappe)
	}
	if config.Dev.Database != "mariadb" {
		t.Errorf("expected dev database mariadb, got %s", config.Dev.Database)
	}

	// Verify dependencies
	if len(config.Dependencies.Apps) != 1 {
		t.Errorf("expected 1 dependency app, got %d", len(config.Dependencies.Apps))
	}
	if config.Dependencies.Apps[0].Name != "payments" {
		t.Errorf("expected dependency name payments, got %s", config.Dependencies.Apps[0].Name)
	}
}

func TestParsePyprojectDefaults(t *testing.T) {
	tmpDir := t.TempDir()

	// Write minimal pyproject.toml
	content := `[project]
name = "test-app"

[tool.weg]
`
	pyprojectPath := filepath.Join(tmpDir, "pyproject.toml")
	if err := os.WriteFile(pyprojectPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	config, err := ParsePyproject(tmpDir)
	if err != nil {
		t.Fatalf("ParsePyproject failed: %v", err)
	}

	// Check defaults are applied
	if len(config.Compatibility.Frappe) != 1 || config.Compatibility.Frappe[0] != "15" {
		t.Errorf("expected default frappe [15], got %v", config.Compatibility.Frappe)
	}
	if len(config.Compatibility.Databases) != 1 || config.Compatibility.Databases[0] != "mariadb" {
		t.Errorf("expected default databases [mariadb], got %v", config.Compatibility.Databases)
	}
	if config.Dev.Frappe != "15" {
		t.Errorf("expected default dev.frappe 15, got %s", config.Dev.Frappe)
	}
	if config.Dev.Database != "mariadb" {
		t.Errorf("expected default dev.database mariadb, got %s", config.Dev.Database)
	}
}

func TestValidateAppConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  AppConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: AppConfig{
				Compatibility: CompatibilityConfig{
					Frappe:    []string{"15"},
					Databases: []string{"mariadb"},
				},
				Dev: DevConfig{
					Frappe:   "15",
					Database: "mariadb",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid frappe version",
			config: AppConfig{
				Compatibility: CompatibilityConfig{
					Frappe:    []string{"99"},
					Databases: []string{"mariadb"},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid database",
			config: AppConfig{
				Compatibility: CompatibilityConfig{
					Frappe:    []string{"15"},
					Databases: []string{"mysql"},
				},
			},
			wantErr: true,
		},
		{
			name: "sqlite without v16",
			config: AppConfig{
				Compatibility: CompatibilityConfig{
					Frappe:    []string{"15"},
					Databases: []string{"sqlite"},
				},
			},
			wantErr: true,
		},
		{
			name: "sqlite with v16",
			config: AppConfig{
				Compatibility: CompatibilityConfig{
					Frappe:    []string{"16"},
					Databases: []string{"sqlite"},
				},
				Dev: DevConfig{
					Frappe:   "16",
					Database: "sqlite",
				},
			},
			wantErr: false,
		},
		{
			name: "dev frappe not in compatibility",
			config: AppConfig{
				Compatibility: CompatibilityConfig{
					Frappe:    []string{"15"},
					Databases: []string{"mariadb"},
				},
				Dev: DevConfig{
					Frappe:   "14",
					Database: "mariadb",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAppConfig(&tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAppConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHasWegSection(t *testing.T) {
	tmpDir := t.TempDir()

	// Test with [tool.weg] section
	withWeg := `[project]
name = "test"

[tool.weg]
[tool.weg.compatibility]
frappe = ["15"]
`
	wegPath := filepath.Join(tmpDir, "with_weg")
	os.MkdirAll(wegPath, 0755)
	os.WriteFile(filepath.Join(wegPath, "pyproject.toml"), []byte(withWeg), 0644)

	if !HasWegSection(wegPath) {
		t.Error("expected HasWegSection to return true")
	}

	// Test without [tool.weg] section
	withoutWeg := `[project]
name = "test"
`
	noWegPath := filepath.Join(tmpDir, "no_weg")
	os.MkdirAll(noWegPath, 0755)
	os.WriteFile(filepath.Join(noWegPath, "pyproject.toml"), []byte(withoutWeg), 0644)

	if HasWegSection(noWegPath) {
		t.Error("expected HasWegSection to return false")
	}
}
