package apps

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseHooksContent_SingleLine(t *testing.T) {
	content := `app_name = "hrms"
app_title = "Frappe HR"
required_apps = ["frappe/erpnext"]
source_link = "http://github.com/frappe/hrms"
`
	deps, err := parseHooksContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deps) != 1 {
		t.Fatalf("expected 1 dep, got %d", len(deps))
	}
	if deps[0].Name != "erpnext" {
		t.Errorf("Name = %q, want %q", deps[0].Name, "erpnext")
	}
	if deps[0].URL != "https://github.com/frappe/erpnext" {
		t.Errorf("URL = %q, want GitHub URL", deps[0].URL)
	}
}

func TestParseHooksContent_MultiLine(t *testing.T) {
	content := `app_name = "my-app"
required_apps = [
    "frappe/erpnext",
    "frappe/hrms",
    "myorg/custom-app",
]
`
	deps, err := parseHooksContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deps) != 3 {
		t.Fatalf("expected 3 deps, got %d", len(deps))
	}

	expected := []struct {
		name string
		url  string
	}{
		{"erpnext", "https://github.com/frappe/erpnext"},
		{"hrms", "https://github.com/frappe/hrms"},
		{"custom-app", "https://github.com/myorg/custom-app"},
	}

	for i, exp := range expected {
		if deps[i].Name != exp.name {
			t.Errorf("deps[%d].Name = %q, want %q", i, deps[i].Name, exp.name)
		}
		if deps[i].URL != exp.url {
			t.Errorf("deps[%d].URL = %q, want %q", i, deps[i].URL, exp.url)
		}
	}
}

func TestParseHooksContent_BareNames(t *testing.T) {
	content := `required_apps = ["erpnext", "hrms"]`
	deps, err := parseHooksContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deps) != 2 {
		t.Fatalf("expected 2 deps, got %d", len(deps))
	}
	// Bare names → no URL
	if deps[0].Name != "erpnext" {
		t.Errorf("deps[0].Name = %q, want %q", deps[0].Name, "erpnext")
	}
	if deps[0].URL != "" {
		t.Errorf("expected empty URL for bare name, got %q", deps[0].URL)
	}
}

func TestParseHooksContent_SkipsFrappe(t *testing.T) {
	content := `required_apps = ["frappe", "frappe/erpnext"]`
	deps, err := parseHooksContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deps) != 1 {
		t.Fatalf("expected 1 dep (frappe skipped), got %d", len(deps))
	}
	if deps[0].Name != "erpnext" {
		t.Errorf("Name = %q, want %q", deps[0].Name, "erpnext")
	}
}

func TestParseHooksContent_NoDeps(t *testing.T) {
	content := `app_name = "myapp"
app_title = "My App"
`
	deps, err := parseHooksContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deps) != 0 {
		t.Errorf("expected 0 deps, got %d", len(deps))
	}
}

func TestParseHooksContent_SingleQuotes(t *testing.T) {
	content := `required_apps = ['frappe/erpnext']`
	deps, err := parseHooksContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deps) != 1 {
		t.Fatalf("expected 1 dep, got %d", len(deps))
	}
	if deps[0].Name != "erpnext" {
		t.Errorf("Name = %q, want %q", deps[0].Name, "erpnext")
	}
}

func TestParsePyprojectContent(t *testing.T) {
	content := `[project]
name = "my-app"
version = "1.0.0"

[tool.weg]
frappe = "15"

[[tool.weg.dependencies.apps]]
name = "erpnext"
url = "https://github.com/frappe/erpnext"
branch = "version-15"

[[tool.weg.dependencies.apps]]
name = "hrms"
url = "https://github.com/frappe/hrms"
branch = "develop"
`
	deps, err := parsePyprojectContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deps) != 2 {
		t.Fatalf("expected 2 deps, got %d", len(deps))
	}
	if deps[0].Name != "erpnext" {
		t.Errorf("deps[0].Name = %q, want %q", deps[0].Name, "erpnext")
	}
	if deps[0].URL != "https://github.com/frappe/erpnext" {
		t.Errorf("deps[0].URL = %q", deps[0].URL)
	}
	if deps[0].Branch != "version-15" {
		t.Errorf("deps[0].Branch = %q, want %q", deps[0].Branch, "version-15")
	}
	if deps[1].Name != "hrms" {
		t.Errorf("deps[1].Name = %q, want %q", deps[1].Name, "hrms")
	}
}

func TestReadDependencies_Hooks(t *testing.T) {
	// Create a temp app directory with hooks.py
	tmpDir := t.TempDir()
	appName := "testapp"
	appPath := filepath.Join(tmpDir, appName)
	appModule := filepath.Join(appPath, appName)
	os.MkdirAll(appModule, 0755)

	hooksContent := `app_name = "testapp"
required_apps = ["frappe/erpnext", "frappe/hrms"]
`
	if err := os.WriteFile(filepath.Join(appModule, "hooks.py"), []byte(hooksContent), 0644); err != nil {
		t.Fatal(err)
	}

	deps, source, err := ReadDependencies(appPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if source != SourceHooks {
		t.Errorf("source = %v, want SourceHooks", source)
	}
	if len(deps) != 2 {
		t.Fatalf("expected 2 deps, got %d", len(deps))
	}
}

func TestReadDependencies_Pyproject(t *testing.T) {
	tmpDir := t.TempDir()
	appPath := filepath.Join(tmpDir, "testapp")
	os.MkdirAll(appPath, 0755)

	pyprojectContent := `[project]
name = "testapp"

[[tool.weg.dependencies.apps]]
name = "erpnext"
url = "https://github.com/frappe/erpnext"
branch = "version-15"
`
	if err := os.WriteFile(filepath.Join(appPath, "pyproject.toml"), []byte(pyprojectContent), 0644); err != nil {
		t.Fatal(err)
	}

	deps, source, err := ReadDependencies(appPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if source != SourcePyproject {
		t.Errorf("source = %v, want SourcePyproject", source)
	}
	if len(deps) != 1 {
		t.Fatalf("expected 1 dep, got %d", len(deps))
	}
}

func TestParseGitHubURL(t *testing.T) {
	tests := []struct {
		url   string
		owner string
		repo  string
		err   bool
	}{
		{"https://github.com/frappe/erpnext", "frappe", "erpnext", false},
		{"https://github.com/frappe/erpnext.git", "frappe", "erpnext", false},
		{"http://github.com/frappe/hrms", "frappe", "hrms", false},
		{"git@github.com:frappe/erpnext.git", "frappe", "erpnext", false},
		{"https://gitlab.com/org/repo", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			owner, repo, err := parseGitHubURL(tt.url)
			if tt.err {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if owner != tt.owner {
				t.Errorf("owner = %q, want %q", owner, tt.owner)
			}
			if repo != tt.repo {
				t.Errorf("repo = %q, want %q", repo, tt.repo)
			}
		})
	}
}
