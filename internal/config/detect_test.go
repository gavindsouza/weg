package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectContext(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(dir string) error
		wantContext   Context
		wantAppName   string
		wantBenchPath bool
	}{
		{
			name:        "fresh directory",
			setup:       func(dir string) error { return nil },
			wantContext: ContextFresh,
		},
		{
			name: "frappe app (hooks.py)",
			setup: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "hooks.py"), []byte("# hooks"), 0644)
			},
			wantContext: ContextApp,
			wantAppName: "", // Will be set to directory name
		},
		{
			name: "traditional bench",
			setup: func(dir string) error {
				if err := os.MkdirAll(filepath.Join(dir, "apps"), 0755); err != nil {
					return err
				}
				return os.MkdirAll(filepath.Join(dir, "sites"), 0755)
			},
			wantContext:   ContextBench,
			wantBenchPath: true,
		},
		{
			name: "weg.toml present",
			setup: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "weg.toml"), []byte("[bench]"), 0644)
			},
			wantContext:   ContextWegBench,
			wantBenchPath: true,
		},
		{
			name: "pyproject.toml with [tool.weg]",
			setup: func(dir string) error {
				content := `[project]
name = "myapp"

[tool.weg]
[tool.weg.compatibility]
frappe = ["15"]
`
				return os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(content), 0644)
			},
			wantContext: ContextWegApp,
		},
		{
			name: ".weg directory with hooks.py",
			setup: func(dir string) error {
				if err := os.WriteFile(filepath.Join(dir, "hooks.py"), []byte("# hooks"), 0644); err != nil {
					return err
				}
				return os.MkdirAll(filepath.Join(dir, ".weg"), 0755)
			},
			wantContext:   ContextWegApp,
			wantBenchPath: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			if err := tt.setup(tmpDir); err != nil {
				t.Fatalf("setup failed: %v", err)
			}

			result, err := DetectContext(tmpDir)
			if err != nil {
				t.Fatalf("DetectContext failed: %v", err)
			}

			if result.Context != tt.wantContext {
				t.Errorf("expected context %v, got %v", tt.wantContext, result.Context)
			}

			if tt.wantBenchPath && result.BenchPath == "" {
				t.Error("expected BenchPath to be set")
			}
		})
	}
}

func TestFindBenchRoot(t *testing.T) {
	tmpDir := t.TempDir()

	// Create bench structure
	benchDir := filepath.Join(tmpDir, "mybench")
	appsDir := filepath.Join(benchDir, "apps")
	sitesDir := filepath.Join(benchDir, "sites")
	appDir := filepath.Join(appsDir, "myapp")

	os.MkdirAll(appsDir, 0755)
	os.MkdirAll(sitesDir, 0755)
	os.MkdirAll(appDir, 0755)

	// Test from app directory
	benchRoot, found := FindBenchRoot(appDir)
	if !found {
		t.Error("expected to find bench root")
	}
	if benchRoot != benchDir {
		t.Errorf("expected bench root %s, got %s", benchDir, benchRoot)
	}

	// Test from bench directory
	benchRoot, found = FindBenchRoot(benchDir)
	if !found {
		t.Error("expected to find bench root")
	}
	if benchRoot != benchDir {
		t.Errorf("expected bench root %s, got %s", benchDir, benchRoot)
	}

	// Test from outside bench
	_, found = FindBenchRoot(tmpDir)
	if found {
		t.Error("expected not to find bench root from outside")
	}
}

func TestFindAppRoot(t *testing.T) {
	tmpDir := t.TempDir()

	// Create app structure
	appDir := filepath.Join(tmpDir, "myapp")
	subDir := filepath.Join(appDir, "myapp", "doctype")

	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(appDir, "hooks.py"), []byte("# hooks"), 0644)

	// Test from subdirectory
	appRoot, found := FindAppRoot(subDir)
	if !found {
		t.Error("expected to find app root")
	}
	if appRoot != appDir {
		t.Errorf("expected app root %s, got %s", appDir, appRoot)
	}

	// Test from app directory
	appRoot, found = FindAppRoot(appDir)
	if !found {
		t.Error("expected to find app root")
	}
	if appRoot != appDir {
		t.Errorf("expected app root %s, got %s", appDir, appRoot)
	}

	// Test from outside app
	_, found = FindAppRoot(tmpDir)
	if found {
		t.Error("expected not to find app root from outside")
	}
}

func TestContextString(t *testing.T) {
	tests := []struct {
		context Context
		want    string
	}{
		{ContextFresh, "fresh"},
		{ContextApp, "frappe-app"},
		{ContextBench, "bench"},
		{ContextWegApp, "weg-app"},
		{ContextWegBench, "weg-bench"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.context.String(); got != tt.want {
				t.Errorf("Context.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetectionResultMethods(t *testing.T) {
	result := &DetectionResult{
		Context: ContextWegApp,
		Path:    "/path/to/app",
		AppName: "myapp",
	}

	desc := result.ContextDescription()
	if desc == "" {
		t.Error("expected non-empty description")
	}

	action := result.SuggestAction()
	if action == "" {
		t.Error("expected non-empty suggested action")
	}
}
