package apps

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveAppSpec_FullURL(t *testing.T) {
	tests := []struct {
		spec   string
		branch string
		name   string
		url    string
	}{
		{"https://github.com/frappe/erpnext", "", "erpnext", "https://github.com/frappe/erpnext"},
		{"https://github.com/frappe/erpnext", "version-15", "erpnext", "https://github.com/frappe/erpnext"},
		{"https://github.com/frappe/erpnext.git", "develop", "erpnext", "https://github.com/frappe/erpnext.git"},
		{"git@github.com:frappe/erpnext.git", "", "erpnext", "git@github.com:frappe/erpnext.git"},
		{"ssh://git@github.com/frappe/erpnext", "", "erpnext", "ssh://git@github.com/frappe/erpnext"},
		{"http://gitlab.com/org/repo", "main", "repo", "http://gitlab.com/org/repo"},
		{"git://github.com/frappe/frappe", "", "frappe", "git://github.com/frappe/frappe"},
	}

	for _, tt := range tests {
		t.Run(tt.spec, func(t *testing.T) {
			spec := ResolveAppSpec(tt.spec, tt.branch)
			if spec.Name != tt.name {
				t.Errorf("Name = %q, want %q", spec.Name, tt.name)
			}
			if spec.URL != tt.url {
				t.Errorf("URL = %q, want %q", spec.URL, tt.url)
			}
			if spec.Branch != tt.branch {
				t.Errorf("Branch = %q, want %q", spec.Branch, tt.branch)
			}
			if spec.IsLocal {
				t.Error("expected IsLocal to be false for git URL")
			}
		})
	}
}

func TestResolveAppSpec_ShortGitHub(t *testing.T) {
	// Force HTTPS protocol for deterministic test results
	origProto := os.Getenv("WEG_GIT_PROTOCOL")
	os.Setenv("WEG_GIT_PROTOCOL", "https")
	ResetCachedProtocol()
	defer func() {
		os.Setenv("WEG_GIT_PROTOCOL", origProto)
		ResetCachedProtocol()
	}()

	tests := []struct {
		spec   string
		branch string
		name   string
		url    string
	}{
		{"frappe/erpnext", "", "erpnext", "https://github.com/frappe/erpnext"},
		{"frappe/erpnext", "version-15", "erpnext", "https://github.com/frappe/erpnext"},
		{"myorg/my-app", "develop", "my-app", "https://github.com/myorg/my-app"},
		{"user/repo.js", "", "repo.js", "https://github.com/user/repo.js"},
	}

	for _, tt := range tests {
		t.Run(tt.spec, func(t *testing.T) {
			spec := ResolveAppSpec(tt.spec, tt.branch)
			if spec.Name != tt.name {
				t.Errorf("Name = %q, want %q", spec.Name, tt.name)
			}
			if spec.URL != tt.url {
				t.Errorf("URL = %q, want %q", spec.URL, tt.url)
			}
			if spec.Branch != tt.branch {
				t.Errorf("Branch = %q, want %q", spec.Branch, tt.branch)
			}
			if spec.IsLocal {
				t.Error("expected IsLocal to be false for short GitHub ref")
			}
		})
	}
}

func TestResolveAppSpec_BareName(t *testing.T) {
	spec := ResolveAppSpec("erpnext", "")
	if spec.Name != "erpnext" {
		t.Errorf("Name = %q, want %q", spec.Name, "erpnext")
	}
	if spec.URL != "" {
		t.Errorf("URL = %q, want empty", spec.URL)
	}
	if spec.IsLocal {
		t.Error("expected IsLocal to be false")
	}

	spec = ResolveAppSpec("erpnext", "version-15")
	if spec.Branch != "version-15" {
		t.Errorf("Branch = %q, want %q", spec.Branch, "version-15")
	}
}

func TestResolveAppSpec_LocalPath(t *testing.T) {
	tmpDir := t.TempDir()
	appDir := filepath.Join(tmpDir, "my-app")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Absolute path
	spec := ResolveAppSpec(appDir, "develop")
	if spec.Name != "my-app" {
		t.Errorf("Name = %q, want %q", spec.Name, "my-app")
	}
	if spec.URL != appDir {
		t.Errorf("URL = %q, want %q", spec.URL, appDir)
	}
	if !spec.IsLocal {
		t.Error("expected IsLocal to be true")
	}
	if spec.Branch != "develop" {
		t.Errorf("Branch = %q, want %q", spec.Branch, "develop")
	}

	// Relative path prefixes
	relSpecs := []string{"./some/path", "../some/path", "~/some/path"}
	for _, s := range relSpecs {
		spec := ResolveAppSpec(s, "")
		if !spec.IsLocal {
			t.Errorf("expected IsLocal to be true for %q", s)
		}
	}
}

func TestResolveAppSpec_ExtractNameFromURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://github.com/frappe/erpnext", "erpnext"},
		{"https://github.com/frappe/erpnext.git", "erpnext"},
		{"https://github.com/frappe/erpnext/", "erpnext"},
		{"git@github.com:frappe/erpnext.git", "erpnext"},
		{"git@gitlab.com:org/my-app.git", "my-app"},
		{"ssh://git@github.com/frappe/frappe", "frappe"},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := extractNameFromURL(tt.url)
			if got != tt.want {
				t.Errorf("extractNameFromURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestAppSpec_String(t *testing.T) {
	tests := []struct {
		spec AppSpec
		want string
	}{
		{AppSpec{Name: "erpnext", URL: "https://github.com/frappe/erpnext", Branch: "develop"}, "erpnext (https://github.com/frappe/erpnext@develop)"},
		{AppSpec{Name: "erpnext", URL: "https://github.com/frappe/erpnext"}, "erpnext (https://github.com/frappe/erpnext)"},
		{AppSpec{Name: "myapp", URL: "/home/user/myapp", IsLocal: true}, "myapp (local: /home/user/myapp)"},
		{AppSpec{Name: "erpnext"}, "erpnext"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.spec.String()
			if got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}
