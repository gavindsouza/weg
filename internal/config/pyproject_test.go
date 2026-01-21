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

func TestCollectAppServices(t *testing.T) {
	// Create temp apps directory structure
	tmpDir := t.TempDir()
	appsDir := filepath.Join(tmpDir, "apps")
	os.MkdirAll(appsDir, 0755)

	// Create app1 with services
	app1Dir := filepath.Join(appsDir, "app1")
	os.MkdirAll(app1Dir, 0755)
	app1Content := `[project]
name = "app1"

[tool.weg.services]
packages = ["tor@latest", "imagemagick@latest"]

[tool.weg.services.processes.tor]
command = "tor -f config/torrc"
depends_on = ["web"]

[tool.weg.services.processes.worker]
command = "python worker.py"
working_dir = "/app"
`
	os.WriteFile(filepath.Join(app1Dir, "pyproject.toml"), []byte(app1Content), 0644)

	// Create app2 with different services
	app2Dir := filepath.Join(appsDir, "app2")
	os.MkdirAll(app2Dir, 0755)
	app2Content := `[project]
name = "app2"

[tool.weg.services]
packages = ["ffmpeg@latest", "tor@latest"]

[tool.weg.services.processes.encoder]
command = "python encoder.py"
`
	os.WriteFile(filepath.Join(app2Dir, "pyproject.toml"), []byte(app2Content), 0644)

	// Create app3 without services (should be skipped)
	app3Dir := filepath.Join(appsDir, "app3")
	os.MkdirAll(app3Dir, 0755)
	app3Content := `[project]
name = "app3"
`
	os.WriteFile(filepath.Join(app3Dir, "pyproject.toml"), []byte(app3Content), 0644)

	// Collect services
	packages, processes, _, err := CollectAppServices(appsDir)
	if err != nil {
		t.Fatalf("CollectAppServices failed: %v", err)
	}

	// Check packages (deduplicated)
	// Should have tor, imagemagick, ffmpeg (tor only once)
	if len(packages) != 3 {
		t.Errorf("expected 3 unique packages, got %d: %v", len(packages), packages)
	}

	packageSet := make(map[string]bool)
	for _, pkg := range packages {
		packageSet[pkg] = true
	}
	if !packageSet["tor@latest"] {
		t.Error("expected tor@latest in packages")
	}
	if !packageSet["imagemagick@latest"] {
		t.Error("expected imagemagick@latest in packages")
	}
	if !packageSet["ffmpeg@latest"] {
		t.Error("expected ffmpeg@latest in packages")
	}

	// Check processes
	if len(processes) != 3 {
		t.Errorf("expected 3 processes, got %d: %v", len(processes), processes)
	}

	if proc, ok := processes["tor"]; !ok {
		t.Error("expected tor process")
	} else {
		if proc.Command != "tor -f config/torrc" {
			t.Errorf("expected tor command, got %s", proc.Command)
		}
		if len(proc.DependsOn) != 1 || proc.DependsOn[0] != "web" {
			t.Errorf("expected tor depends_on [web], got %v", proc.DependsOn)
		}
	}

	if proc, ok := processes["worker"]; !ok {
		t.Error("expected worker process")
	} else {
		if proc.WorkingDir != "/app" {
			t.Errorf("expected worker working_dir /app, got %s", proc.WorkingDir)
		}
	}

	if _, ok := processes["encoder"]; !ok {
		t.Error("expected encoder process")
	}
}

func TestCollectAppServicesWithSymlinks(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()
	appsDir := filepath.Join(tmpDir, "apps")
	os.MkdirAll(appsDir, 0755)

	// Create actual app directory outside of apps/
	realAppDir := filepath.Join(tmpDir, "real_app")
	os.MkdirAll(realAppDir, 0755)
	appContent := `[project]
name = "symlinked-app"

[tool.weg.services]
packages = ["redis-stack@latest"]

[tool.weg.services.processes.redis-worker]
command = "redis-worker"
`
	os.WriteFile(filepath.Join(realAppDir, "pyproject.toml"), []byte(appContent), 0644)

	// Create symlink in apps/
	symlinkPath := filepath.Join(appsDir, "symlinked_app")
	if err := os.Symlink(realAppDir, symlinkPath); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	// Collect services
	packages, processes, _, err := CollectAppServices(appsDir)
	if err != nil {
		t.Fatalf("CollectAppServices failed: %v", err)
	}

	// Should find the symlinked app's services
	if len(packages) != 1 || packages[0] != "redis-stack@latest" {
		t.Errorf("expected [redis-stack@latest], got %v", packages)
	}

	if _, ok := processes["redis-worker"]; !ok {
		t.Error("expected redis-worker process from symlinked app")
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
