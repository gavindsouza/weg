package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewBenchBasic(t *testing.T) {
	bench := NewBench(t).
		WithApp("frappe").
		WithApp("erpnext").
		WithSite("test.localhost").
		Build()

	// weg.toml should exist
	if _, err := os.Stat(filepath.Join(bench, "weg.toml")); err != nil {
		t.Error("weg.toml should exist")
	}

	// Apps should exist
	for _, app := range []string{"frappe", "erpnext"} {
		if _, err := os.Stat(filepath.Join(bench, "apps", app)); err != nil {
			t.Errorf("app dir %s should exist", app)
		}
	}

	// Site should exist with config
	cfgPath := filepath.Join(bench, "sites", "test.localhost", "site_config.json")
	if _, err := os.Stat(cfgPath); err != nil {
		t.Error("site_config.json should exist")
	}

	// Assets dir should exist
	if _, err := os.Stat(filepath.Join(bench, "sites", "assets")); err != nil {
		t.Error("sites/assets should exist")
	}
}

func TestNewBenchCustomWegToml(t *testing.T) {
	content := "[frappe]\nversion = \"16\"\n"
	bench := NewBench(t).WithWegToml(content).Build()

	data, err := os.ReadFile(filepath.Join(bench, "weg.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != content {
		t.Errorf("weg.toml content = %q, want %q", data, content)
	}
}

func TestNewBenchWithState(t *testing.T) {
	stateJSON := `{"version":"1","apps":{},"sites":{}}`
	bench := NewBench(t).WithState(stateJSON).Build()

	statePath := filepath.Join(bench, ".weg", "state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != stateJSON {
		t.Errorf("state.json content = %q, want %q", data, stateJSON)
	}
}

func TestNewBenchChdir(t *testing.T) {
	b := NewBench(t).Build()
	oldWd, _ := os.Getwd()

	bench := NewBench(t)
	bench.path = b // reuse the already-built path
	bench.t = t
	bench.Chdir()

	wd, _ := os.Getwd()
	if wd != b {
		t.Errorf("working dir = %q, want %q", wd, b)
	}

	// After test cleanup, we'd be back. We can't easily test t.Cleanup here,
	// but we verify the chdir worked.
	_ = oldWd
}

func TestNewBenchEmpty(t *testing.T) {
	// Bench with no apps or sites should still create weg.toml
	bench := NewBench(t).Build()
	if _, err := os.Stat(filepath.Join(bench, "weg.toml")); err != nil {
		t.Error("empty bench should still have weg.toml")
	}
}
