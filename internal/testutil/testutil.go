// Package testutil provides shared test helpers for the weg CLI.
package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// Bench builds a temporary bench directory structure for tests.
type Bench struct {
	t    *testing.T
	path string
	apps []string
	// sites maps site name to its config content
	sites      map[string]string
	wegToml    string
	stateJSON  string
}

// NewBench creates a test bench builder. Call Build() to materialize.
func NewBench(t *testing.T) *Bench {
	t.Helper()
	return &Bench{
		t:     t,
		sites: make(map[string]string),
	}
}

// WithApp adds an app directory to the bench.
func (b *Bench) WithApp(name string) *Bench {
	b.apps = append(b.apps, name)
	return b
}

// WithSite adds a site with a default site_config.json.
func (b *Bench) WithSite(name string) *Bench {
	b.sites[name] = `{"db_name": "test"}`
	return b
}

// WithSiteConfig adds a site with custom site_config.json content.
func (b *Bench) WithSiteConfig(name, configJSON string) *Bench {
	b.sites[name] = configJSON
	return b
}

// WithWegToml sets custom weg.toml content. If not called, a default is used.
func (b *Bench) WithWegToml(content string) *Bench {
	b.wegToml = content
	return b
}

// WithState sets custom .weg/state.json content.
func (b *Bench) WithState(stateJSON string) *Bench {
	b.stateJSON = stateJSON
	return b
}

// Build materializes the bench directory and returns its path.
func (b *Bench) Build() string {
	b.t.Helper()
	b.path = b.t.TempDir()

	// Default weg.toml
	wegContent := b.wegToml
	if wegContent == "" {
		wegContent = "[frappe]\nversion = \"15\"\n"
	}
	b.writeFile("weg.toml", wegContent)

	// Create apps
	for _, app := range b.apps {
		b.mkdir(filepath.Join("apps", app))
	}

	// Create sites with config files
	b.mkdir("sites/assets") // standard bench assets dir
	for name, cfg := range b.sites {
		siteDir := filepath.Join("sites", name)
		b.mkdir(siteDir)
		b.writeFile(filepath.Join(siteDir, "site_config.json"), cfg)
	}

	// Create state if specified
	if b.stateJSON != "" {
		b.mkdir(".weg")
		b.writeFile(filepath.Join(".weg", "state.json"), b.stateJSON)
	}

	return b.path
}

// Path returns the bench directory path. Must be called after Build().
func (b *Bench) Path() string {
	return b.path
}

func (b *Bench) mkdir(rel string) {
	b.t.Helper()
	if err := os.MkdirAll(filepath.Join(b.path, rel), 0755); err != nil {
		b.t.Fatalf("failed to create dir %s: %v", rel, err)
	}
}

func (b *Bench) writeFile(rel, content string) {
	b.t.Helper()
	full := filepath.Join(b.path, rel)
	if err := os.WriteFile(full, []byte(content), 0644); err != nil {
		b.t.Fatalf("failed to write %s: %v", rel, err)
	}
}

// Chdir changes to the bench directory and restores on cleanup.
func (b *Bench) Chdir() {
	b.t.Helper()
	old, err := os.Getwd()
	if err != nil {
		b.t.Fatalf("failed to get working dir: %v", err)
	}
	if err := os.Chdir(b.path); err != nil {
		b.t.Fatalf("failed to chdir to bench: %v", err)
	}
	b.t.Cleanup(func() { os.Chdir(old) })
}
