package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetVenvPython(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(dir string)
		wantVenv bool
		wantFall string
	}{
		{
			name: "venv exists",
			setup: func(dir string) {
				venvBin := filepath.Join(dir, "env", "bin")
				os.MkdirAll(venvBin, 0755)
				os.WriteFile(filepath.Join(venvBin, "python"), []byte("#!/bin/bash\n"), 0755)
			},
			wantVenv: true,
		},
		{
			name:     "no venv falls back to python3",
			setup:    func(dir string) {},
			wantVenv: false,
			wantFall: "python3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(dir)

			result := GetVenvPython(dir)

			if tt.wantVenv {
				expected := filepath.Join(dir, "env", "bin", "python")
				if result != expected {
					t.Errorf("GetVenvPython() = %v, want %v", result, expected)
				}
			} else {
				if result != tt.wantFall {
					t.Errorf("GetVenvPython() = %v, want %v", result, tt.wantFall)
				}
			}
		})
	}
}

func TestHasDevbox(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(dir string)
		expect bool
	}{
		{
			name: "devbox.json exists",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "devbox.json"), []byte("{}"), 0644)
			},
			expect: true,
		},
		{
			name:   "no devbox.json",
			setup:  func(dir string) {},
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(dir)

			result := HasDevbox(dir)
			if result != tt.expect {
				t.Errorf("HasDevbox() = %v, want %v", result, tt.expect)
			}
		})
	}
}

func TestBuildPythonPath(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(dir string)
		wantCount int
	}{
		{
			name: "multiple apps",
			setup: func(dir string) {
				appsDir := filepath.Join(dir, "apps")
				os.MkdirAll(filepath.Join(appsDir, "frappe"), 0755)
				os.MkdirAll(filepath.Join(appsDir, "erpnext"), 0755)
				os.MkdirAll(filepath.Join(appsDir, "myapp"), 0755)
			},
			wantCount: 3,
		},
		{
			name: "single app",
			setup: func(dir string) {
				appsDir := filepath.Join(dir, "apps")
				os.MkdirAll(filepath.Join(appsDir, "frappe"), 0755)
			},
			wantCount: 1,
		},
		{
			name:      "no apps directory",
			setup:     func(dir string) {},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(dir)

			pathStr, paths := BuildPythonPath(dir)

			if len(paths) != tt.wantCount {
				t.Errorf("BuildPythonPath() paths count = %d, want %d", len(paths), tt.wantCount)
			}

			if tt.wantCount > 0 {
				if pathStr == "" {
					t.Error("BuildPythonPath() returned empty path string")
				}
				// Verify all paths are in the string
				for _, p := range paths {
					if !strings.Contains(pathStr, p) {
						t.Errorf("BuildPythonPath() path string missing %s", p)
					}
				}
			}
		})
	}
}

func TestMergePythonPath(t *testing.T) {
	tests := []struct {
		name     string
		newPath  string
		existing string
		want     string
	}{
		{
			name:     "empty new path",
			newPath:  "",
			existing: "/existing/path",
			want:     "/existing/path",
		},
		{
			name:     "new path only",
			newPath:  "/new/path",
			existing: "",
			want:     "/new/path",
		},
		{
			name:     "merge paths",
			newPath:  "/new/path",
			existing: "/existing/path",
			want:     "/new/path:/existing/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set existing PYTHONPATH
			oldPath := os.Getenv("PYTHONPATH")
			defer os.Setenv("PYTHONPATH", oldPath)
			os.Setenv("PYTHONPATH", tt.existing)

			result := MergePythonPath(tt.newPath)
			if result != tt.want {
				t.Errorf("MergePythonPath() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestSitesDir(t *testing.T) {
	benchPath := "/path/to/bench"
	result := SitesDir(benchPath)
	expected := "/path/to/bench/sites"
	if result != expected {
		t.Errorf("SitesDir() = %v, want %v", result, expected)
	}
}

func TestAppsDir(t *testing.T) {
	benchPath := "/path/to/bench"
	result := AppsDir(benchPath)
	expected := "/path/to/bench/apps"
	if result != expected {
		t.Errorf("AppsDir() = %v, want %v", result, expected)
	}
}

func TestRequireSite(t *testing.T) {
	tests := []struct {
		name    string
		site    string
		wantErr bool
	}{
		{
			name:    "empty site returns error",
			site:    "",
			wantErr: true,
		},
		{
			name:    "valid site returns nil",
			site:    "mysite.localhost",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RequireSite(tt.site)
			if (err != nil) != tt.wantErr {
				t.Errorf("RequireSite() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildBenchEnv(t *testing.T) {
	benchPath := "/path/to/bench"
	site := "mysite.localhost"

	env := BuildBenchEnv(benchPath, site)

	// Check that required vars are present
	var hasBenchRoot, hasSite bool
	for _, e := range env {
		if e == "FRAPPE_BENCH_ROOT=/path/to/bench" {
			hasBenchRoot = true
		}
		if e == "FRAPPE_SITE=mysite.localhost" {
			hasSite = true
		}
	}

	if !hasBenchRoot {
		t.Error("BuildBenchEnv() missing FRAPPE_BENCH_ROOT")
	}
	if !hasSite {
		t.Error("BuildBenchEnv() missing FRAPPE_SITE")
	}
}

func TestBuildBenchEnv_NoSite(t *testing.T) {
	benchPath := "/path/to/bench"

	env := BuildBenchEnv(benchPath, "")

	// Check that FRAPPE_SITE is NOT present when empty
	for _, e := range env {
		if strings.HasPrefix(e, "FRAPPE_SITE=") {
			t.Error("BuildBenchEnv() should not include FRAPPE_SITE when empty")
		}
	}
}

func TestResolveDefaultSiteWithFallback(t *testing.T) {
	tests := []struct {
		name  string
		setup func(dir string)
		want  string
	}{
		{
			name: "finds site directory",
			setup: func(dir string) {
				sitesDir := filepath.Join(dir, "sites")
				os.MkdirAll(filepath.Join(sitesDir, "mysite.localhost"), 0755)
			},
			want: "mysite.localhost",
		},
		{
			name: "ignores hidden directories",
			setup: func(dir string) {
				sitesDir := filepath.Join(dir, "sites")
				os.MkdirAll(filepath.Join(sitesDir, ".hidden"), 0755)
				os.MkdirAll(filepath.Join(sitesDir, "realsite.localhost"), 0755)
			},
			want: "realsite.localhost",
		},
		{
			name: "ignores assets directory",
			setup: func(dir string) {
				sitesDir := filepath.Join(dir, "sites")
				os.MkdirAll(filepath.Join(sitesDir, "assets"), 0755)
				os.MkdirAll(filepath.Join(sitesDir, "mysite.localhost"), 0755)
			},
			want: "mysite.localhost",
		},
		{
			name:  "no sites returns empty",
			setup: func(dir string) {},
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(dir)

			result := ResolveDefaultSiteWithFallback(dir)
			if result != tt.want {
				t.Errorf("ResolveDefaultSiteWithFallback() = %v, want %v", result, tt.want)
			}
		})
	}
}
