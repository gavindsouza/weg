package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gavindsouza/weg/internal/config"
)

const ciMatrixFixture = `name: CI

on:
  push:
    branches: [main, develop]
  pull_request:
  schedule:
    - cron: "0 0 * * 5"

jobs:
  test:
    runs-on: ubuntu-latest
    strategy:
      fail-fast: false
      matrix:
        include:
          - {version: "15", python-version: "3.11", node-version: 18}
          - {version: "16", python-version: "3.14", node-version: 24, frappe-branch: "version-16"}
          - {version: "develop", python-version: "3.14", node-version: 22}

    services:
      mariadb:
        image: mariadb:10.11
      redis-cache:
        image: redis:alpine
`

func writeFixtureRepo(t *testing.T, workflowName string, content string) string {
	t.Helper()
	repo := t.TempDir()
	dir := filepath.Join(repo, ".github", "workflows")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create workflows dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, workflowName), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write workflow: %v", err)
	}
	return repo
}

func TestDetectCICompatibility_Matrix(t *testing.T) {
	repo := writeFixtureRepo(t, "ci.yml", ciMatrixFixture)

	versions, databases := detectCICompatibility(repo)

	if want := []string{"15", "16", "develop"}; !reflect.DeepEqual(versions, want) {
		t.Errorf("expected versions %v, got %v", want, versions)
	}
	if want := []string{"mariadb"}; !reflect.DeepEqual(databases, want) {
		t.Errorf("expected databases %v, got %v", want, databases)
	}
}

func TestDetectCICompatibility_StandaloneStyle(t *testing.T) {
	content := `jobs:
  test:
    strategy:
      matrix:
        include:
          - python-version: "3.10"
          - python-version: "3.11"
    services:
      postgres:
        image: postgres:15
      mariadb:
        image: mariadb:10.11
`
	repo := writeFixtureRepo(t, "ci.yml", content)

	versions, databases := detectCICompatibility(repo)

	if want := []string{"14", "15"}; !reflect.DeepEqual(versions, want) {
		t.Errorf("expected versions %v, got %v", want, versions)
	}
	if want := []string{"mariadb", "postgres"}; !reflect.DeepEqual(databases, want) {
		t.Errorf("expected databases %v, got %v", want, databases)
	}
}

func TestDetectCICompatibility_NoWorkflows(t *testing.T) {
	repo := t.TempDir()
	versions, databases := detectCICompatibility(repo)
	if versions != nil || databases != nil {
		t.Errorf("expected nil for empty repo, got versions=%v databases=%v", versions, databases)
	}
}

func TestDetectCICompatibility_IgnoresMatrixRefs(t *testing.T) {
	content := `jobs:
  test:
    steps:
      - uses: actions/setup-python@v5
        with:
          python-version: ${{ matrix.python-version }}
`
	repo := writeFixtureRepo(t, "ci.yml", content)
	versions, _ := detectCICompatibility(repo)
	if versions != nil {
		t.Errorf("expected no versions from matrix references, got %v", versions)
	}
}

func TestDetectCICompatibility_SheetsStyle(t *testing.T) {
	// sheets uses underscore keys, a bare list, and version-prefixed values.
	content := `jobs:
  tests:
    runs-on: ubuntu-latest
    strategy:
      fail-fast: false
      matrix:
        frappe_version: [version-15, version-16, develop]
        include:
          - frappe_version: version-15
            python_version: '3.11'
            node_version: 18
          - frappe_version: version-16
            python_version: '3.14'
            node_version: 24
          - frappe_version: develop
            python_version: '3.14'
            node_version: 24
    services:
      mariadb:
        image: mariadb:11.8
      redis-cache:
        image: redis:alpine
`
	repo := writeFixtureRepo(t, "ci.yml", content)

	versions, databases := detectCICompatibility(repo)

	if want := []string{"15", "16", "develop"}; !reflect.DeepEqual(versions, want) {
		t.Errorf("expected versions %v, got %v", want, versions)
	}
	if want := []string{"mariadb"}; !reflect.DeepEqual(databases, want) {
		t.Errorf("expected databases %v, got %v", want, databases)
	}
}

func TestBuildMatrixInclude(t *testing.T) {
	got, err := buildMatrixInclude([]string{"15", "16", "develop"})
	if err != nil {
		t.Fatalf("buildMatrixInclude failed: %v", err)
	}

	for _, want := range []string{
		`{version: "15", python-version: "3.11", node-version: "18", frappe-branch: "version-15"}`,
		`{version: "16", python-version: "3.14", node-version: "24", frappe-branch: "version-16"}`,
		`{version: "develop", python-version: "3.14", node-version: "24", frappe-branch: "develop"}`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("matrix include missing %s, got:\n%s", want, got)
		}
	}
}

func TestBuildMatrixInclude_UnsupportedVersion(t *testing.T) {
	if _, err := buildMatrixInclude([]string{"99"}); err == nil {
		t.Error("expected error for unsupported version, got nil")
	}
}

func TestWegSection_Add(t *testing.T) {
	original := "[project]\nname = \"my-app\"\n"
	section := "[tool.weg]\n[tool.weg.compatibility]\n"

	got := addWegSection(original, section)
	if !strings.Contains(got, original) || !strings.Contains(got, "[tool.weg]") {
		t.Errorf("addWegSection result missing content:\n%s", got)
	}
	if !strings.HasSuffix(got, section) {
		t.Errorf("addWegSection should append at end:\n%s", got)
	}
}

func TestWegSection_FindReplace(t *testing.T) {
	original := `[project]
name = "my-app"

[tool.weg]
[tool.weg.compatibility]
frappe = ["15"]
databases = ["mariadb"]

[tool.weg.dev]
frappe = "15"
database = "mariadb"

[tool.frappe]
latest = true
`
	found := findWegSection(original)
	if !strings.Contains(found, `frappe = ["15"]`) || strings.Contains(found, "[tool.frappe]") {
		t.Errorf("findWegSection should stop at next table, got:\n%s", found)
	}

	replacement := buildWegSection(&bootstrapPlan{
		versions:   []string{"15", "16"},
		databases:  []string{"mariadb"},
		devVersion: "15",
		devDB:      "mariadb",
	})
	got := replaceWegSection(original, replacement)

	if strings.Contains(got, `frappe = ["15"]`) {
		t.Errorf("replaceWegSection left old config behind:\n%s", got)
	}
	if !strings.Contains(got, `frappe = ["15", "16"]`) {
		t.Errorf("replaceWegSection did not apply new config:\n%s", got)
	}
	if !strings.Contains(got, "[tool.frappe]") {
		t.Errorf("replaceWegSection dropped unrelated section:\n%s", got)
	}
	if strings.Contains(got, "[tool.weg]\n[tool.weg.compatibility]") {
		t.Log("[tool.weg] section present once, blank separator retained")
	}
}

func TestWegSection_RoundTripGenerated(t *testing.T) {
	plan := &bootstrapPlan{
		versions:   []string{"15", "16", "develop"},
		databases:  []string{"mariadb"},
		devVersion: "15",
		devDB:      "mariadb",
	}
	section := buildWegSection(plan)
	content := "[project]\nname = \"x\"\n" + section

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write pyproject: %v", err)
	}

	// Parsing the written file must round-trip back to the same matrix.
	cfg, err := config.ParsePyproject(dir)
	if err != nil {
		t.Fatalf("failed to parse generated config: %v", err)
	}
	if !reflect.DeepEqual(cfg.Compatibility.Frappe, []string{"15", "16", "develop"}) {
		t.Errorf("round-trip versions = %v", cfg.Compatibility.Frappe)
	}
}

func TestQuotedList(t *testing.T) {
	got := quotedList([]string{"15", "16", "develop"})
	if want := `"15", "16", "develop"`; got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestParseCSV(t *testing.T) {
	cases := map[string][]string{
		"15,16,develop":      {"15", "16", "develop"},
		" 15 , 16 ,develop ": {"15", "16", "develop"},
		"":                   nil,
		"   ":                nil,
	}
	for input, want := range cases {
		if got := parseCSV(input); !reflect.DeepEqual(got, want) {
			t.Errorf("parseCSV(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestFrappeBranch(t *testing.T) {
	if got := frappeBranch("15"); got != "version-15" {
		t.Errorf("frappeBranch(15) = %q", got)
	}
	if got := frappeBranch("develop"); got != "develop" {
		t.Errorf("frappeBranch(develop) = %q", got)
	}
}

func TestGetDevboxPackages_Develop(t *testing.T) {
	packages := getDevboxPackages("develop")
	joined := strings.Join(packages, " ")
	for _, want := range []string{"python@3.14", "nodejs@24", "pnpm"} {
		if !strings.Contains(joined, want) {
			t.Errorf("devbox packages for develop missing %q, got %v", want, packages)
		}
	}
}

func TestGetDevboxPackages_V15(t *testing.T) {
	packages := getDevboxPackages("15")
	for _, want := range []string{"python@3.11", "nodejs@18", "yarn"} {
		found := false
		for _, p := range packages {
			if p == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("devbox packages for 15 missing %q, got %v", want, packages)
		}
	}
}

func TestRenderCIWorkflow(t *testing.T) {
	plan := &bootstrapPlan{
		moduleName: "sheets_sync",
		versions:   []string{"15", "16", "develop"},
	}
	content, err := renderCIWorkflow(plan)
	if err != nil {
		t.Fatalf("renderCIWorkflow failed: %v", err)
	}

	for _, want := range []string{
		`cron: "0 0 * * 5"`,
		"workflow_dispatch:",
		`{version: "15", python-version: "3.11",`,
		`{version: "develop", python-version: "3.14",`,
		"sheets_sync",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("rendered CI missing %q:\n%s", want, content)
		}
	}
}

func TestRenderWegToml(t *testing.T) {
	plan := &bootstrapPlan{
		appName:    "sheets",
		moduleName: "sheets_sync",
		versions:   []string{"15", "16", "develop"},
		devVersion: "16",
		devDB:      "mariadb",
		siteName:   "sheets_sync.localhost",
	}
	content, err := renderWegToml(plan)
	if err != nil {
		t.Fatalf("renderWegToml failed: %v", err)
	}

	for _, want := range []string{
		`version = "16"`,
		`branch = "version-16"`,
		`name = "sheets_sync.localhost"`,
		`apps = ["frappe", "sheets_sync"]`,
	} {
		if !strings.Contains(content, want) {
			t.Errorf("rendered weg.toml missing %q:\n%s", want, content)
		}
	}
}
func TestFrappeDependencyRange(t *testing.T) {
	cases := []struct {
		label    string
		versions []string
		want     string
	}{
		{"single stable", []string{"15"}, ">=15.0.0,<16.0.0"},
		{"two stables", []string{"15", "16"}, ">=15.0.0,<17.0.0"},
		{"stable with develop", []string{"15", "16", "develop"}, ">=15.0.0,<18.0.0"},
		{"develop only", []string{"develop"}, ">=17.0.0,<18.0.0"},
		{"wide range", []string{"13", "14", "15", "16", "develop"}, ">=13.0.0,<18.0.0"},
		{"v prefix", []string{"v14", "15"}, ">=14.0.0,<16.0.0"},
		{"empty", nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			if got := frappeDependencyRange(tc.versions); got != tc.want {
				t.Errorf("frappeDependencyRange(%v) = %q, want %q", tc.versions, got, tc.want)
			}
		})
	}
}

func TestHasBenchDependencies(t *testing.T) {
	content := "[project]\nname = \"x\"\n\n[tool.bench.frappe-dependencies]\nfrappe = \">=15.0.0,<18.0.0\"\n"
	if !hasBenchDependencies(content) {
		t.Error("expected bench dependencies to be detected")
	}
	if hasBenchDependencies("[tool.weg]\n[tool.weg.dev]\nfrappe = \"15\"\n") {
		t.Error("unexpected bench dependencies detection")
	}
}
