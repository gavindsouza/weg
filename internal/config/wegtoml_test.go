package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseWegToml(t *testing.T) {
	tmpDir := t.TempDir()

	content := `[bench]
name = "test-bench"

[frappe]
version = "15"
database = "mariadb"

[apps.frappe]
url = "https://github.com/frappe/frappe"
branch = "version-15"

[apps.erpnext]
url = "https://github.com/frappe/erpnext"
branch = "version-15"

[[sites]]
name = "test.localhost"
default = true
apps = ["frappe", "erpnext"]

[[sites]]
name = "dev.localhost"
apps = ["frappe"]
`
	wegPath := filepath.Join(tmpDir, "weg.toml")
	if err := os.WriteFile(wegPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	config, err := ParseWegToml(tmpDir)
	if err != nil {
		t.Fatalf("ParseWegToml failed: %v", err)
	}

	// Verify bench settings
	if config.Bench.Name != "test-bench" {
		t.Errorf("expected bench name test-bench, got %s", config.Bench.Name)
	}

	// Verify frappe settings
	if config.Frappe.Version != "15" {
		t.Errorf("expected frappe version 15, got %s", config.Frappe.Version)
	}
	if config.Frappe.Database != "mariadb" {
		t.Errorf("expected database mariadb, got %s", config.Frappe.Database)
	}

	// Verify apps
	if len(config.Apps) != 2 {
		t.Errorf("expected 2 apps, got %d", len(config.Apps))
	}
	if frappe, ok := config.Apps["frappe"]; !ok {
		t.Error("expected frappe app to exist")
	} else if frappe.Branch != "version-15" {
		t.Errorf("expected frappe branch version-15, got %s", frappe.Branch)
	}

	// Verify sites
	if len(config.Sites) != 2 {
		t.Errorf("expected 2 sites, got %d", len(config.Sites))
	}
	if config.Sites[0].Name != "test.localhost" {
		t.Errorf("expected first site test.localhost, got %s", config.Sites[0].Name)
	}
	if !config.Sites[0].DefaultSite {
		t.Error("expected first site to be default")
	}
}

func TestParseWegTomlDefaults(t *testing.T) {
	tmpDir := t.TempDir()

	// Minimal weg.toml
	content := `[bench]
name = "minimal"
`
	wegPath := filepath.Join(tmpDir, "weg.toml")
	if err := os.WriteFile(wegPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	config, err := ParseWegToml(tmpDir)
	if err != nil {
		t.Fatalf("ParseWegToml failed: %v", err)
	}

	// Check defaults
	if config.Frappe.Version != "15" {
		t.Errorf("expected default version 15, got %s", config.Frappe.Version)
	}
	if config.Frappe.Database != "mariadb" {
		t.Errorf("expected default database mariadb, got %s", config.Frappe.Database)
	}

	// Check frappe is added to apps
	if _, ok := config.Apps["frappe"]; !ok {
		t.Error("expected frappe to be added to apps by default")
	}

	// Check service defaults
	if config.Services.Web.Port != 8000 {
		t.Errorf("expected default web port 8000, got %d", config.Services.Web.Port)
	}
}

func TestValidateBenchConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  BenchConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: BenchConfig{
				Frappe: FrappeSettings{
					Version:  "15",
					Database: "mariadb",
				},
				Apps: map[string]AppSettings{
					"frappe": {URL: "https://github.com/frappe/frappe", Branch: "version-15"},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid version",
			config: BenchConfig{
				Frappe: FrappeSettings{
					Version:  "99",
					Database: "mariadb",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid database",
			config: BenchConfig{
				Frappe: FrappeSettings{
					Version:  "15",
					Database: "mysql",
				},
			},
			wantErr: true,
		},
		{
			name: "sqlite without v16",
			config: BenchConfig{
				Frappe: FrappeSettings{
					Version:  "15",
					Database: "sqlite",
				},
			},
			wantErr: true,
		},
		{
			name: "sqlite with v16",
			config: BenchConfig{
				Frappe: FrappeSettings{
					Version:  "16",
					Database: "sqlite",
				},
				Apps: map[string]AppSettings{
					"frappe": {URL: "https://github.com/frappe/frappe"},
				},
			},
			wantErr: false,
		},
		{
			name: "app without url or path",
			config: BenchConfig{
				Frappe: FrappeSettings{
					Version:  "15",
					Database: "mariadb",
				},
				Apps: map[string]AppSettings{
					"frappe":  {URL: "https://github.com/frappe/frappe"},
					"someapp": {}, // No URL or path
				},
			},
			wantErr: true,
		},
		{
			name: "multiple default sites",
			config: BenchConfig{
				Frappe: FrappeSettings{
					Version:  "15",
					Database: "mariadb",
				},
				Apps: map[string]AppSettings{
					"frappe": {URL: "https://github.com/frappe/frappe"},
				},
				Sites: []SiteConfig{
					{Name: "site1", DefaultSite: true},
					{Name: "site2", DefaultSite: true},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBenchConfig(&tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBenchConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBenchConfigMethods(t *testing.T) {
	config := BenchConfig{
		Apps: map[string]AppSettings{
			"frappe":  {URL: "https://github.com/frappe/frappe", Branch: "version-15"},
			"erpnext": {URL: "https://github.com/frappe/erpnext", Branch: "version-15"},
			"custom":  {URL: "https://github.com/org/custom", Excluded: true},
		},
		Sites: []SiteConfig{
			{Name: "site1", DefaultSite: false},
			{Name: "site2", DefaultSite: true},
		},
	}

	// Test GetApp
	app, ok := config.GetApp("frappe")
	if !ok {
		t.Error("expected to find frappe app")
	}
	if app.Branch != "version-15" {
		t.Errorf("expected frappe branch version-15, got %s", app.Branch)
	}

	_, ok = config.GetApp("nonexistent")
	if ok {
		t.Error("expected not to find nonexistent app")
	}

	// Test GetDefaultSite
	defaultSite := config.GetDefaultSite()
	if defaultSite == nil {
		t.Fatal("expected to find default site")
	}
	if defaultSite.Name != "site2" {
		t.Errorf("expected default site site2, got %s", defaultSite.Name)
	}

	// Test EnabledApps
	enabled := config.EnabledApps()
	if len(enabled) != 2 {
		t.Errorf("expected 2 enabled apps, got %d", len(enabled))
	}
	if _, ok := enabled["custom"]; ok {
		t.Error("excluded app should not be in enabled apps")
	}

	// Test AppNames
	names := config.AppNames()
	if len(names) != 3 {
		t.Errorf("expected 3 app names, got %d", len(names))
	}
}

func TestParseWegTomlWorkerConfig(t *testing.T) {
	tmpDir := t.TempDir()

	content := `[bench]
name = "test-bench"

[frappe]
version = "15"
database = "mariadb"

[services.workers]
short = 1
default = 2
long = 1
`
	wegPath := filepath.Join(tmpDir, "weg.toml")
	if err := os.WriteFile(wegPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	config, err := ParseWegToml(tmpDir)
	if err != nil {
		t.Fatalf("ParseWegToml failed: %v", err)
	}

	// Verify worker config
	if config.Services.Workers["short"] != 1 {
		t.Errorf("expected short workers 1, got %d", config.Services.Workers["short"])
	}
	if config.Services.Workers["default"] != 2 {
		t.Errorf("expected default workers 2, got %d", config.Services.Workers["default"])
	}
	if config.Services.Workers["long"] != 1 {
		t.Errorf("expected long workers 1, got %d", config.Services.Workers["long"])
	}
}

func TestParseWegTomlWorkerConfigCustomQueues(t *testing.T) {
	tmpDir := t.TempDir()

	content := `[bench]
name = "test-bench"

[frappe]
version = "15"
database = "mariadb"

[services.workers]
all = 1
notifications = 2
exports = 1
`
	wegPath := filepath.Join(tmpDir, "weg.toml")
	if err := os.WriteFile(wegPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	config, err := ParseWegToml(tmpDir)
	if err != nil {
		t.Fatalf("ParseWegToml failed: %v", err)
	}

	// Verify worker config - custom queues
	if config.Services.Workers["all"] != 1 {
		t.Errorf("expected all workers 1, got %d", config.Services.Workers["all"])
	}
	if config.Services.Workers["notifications"] != 2 {
		t.Errorf("expected notifications workers 2, got %d", config.Services.Workers["notifications"])
	}
	if config.Services.Workers["exports"] != 1 {
		t.Errorf("expected exports workers 1, got %d", config.Services.Workers["exports"])
	}
}
