package tests

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	// Importing cmd registers every subcommand on the root "weg" command,
	// so help output here matches the real binary (usage paths, global flags).
	_ "github.com/gavindsouza/weg/cmd"
	"github.com/gavindsouza/weg/cmd/site"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/spf13/cobra"
)

var update = flag.Bool("update", false, "update golden files")

// assertGolden compares got against a golden file. If -update flag is set, updates the file.
func assertGolden(t *testing.T, name, got string) {
	t.Helper()

	goldenPath := filepath.Join("testdata", name+".golden")

	if *update {
		os.MkdirAll(filepath.Dir(goldenPath), 0755)
		if err := os.WriteFile(goldenPath, []byte(got), 0644); err != nil {
			t.Fatalf("failed to update golden file: %v", err)
		}
		return
	}

	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("golden file %s not found (run with -update to create): %v", goldenPath, err)
	}

	if got != string(expected) {
		t.Errorf("output does not match golden file %s\n\ngot:\n%s\n\nwant:\n%s", goldenPath, got, string(expected))
	}
}

// rootCommand returns the fully-wired root "weg" command.
func rootCommand(t *testing.T) *cobra.Command {
	t.Helper()
	root := site.SiteCmd.Root()
	if root.Name() != "weg" {
		t.Fatalf("expected site command to be attached to weg root, got %q", root.Name())
	}
	return root
}

// captureHelp runs `weg <path...> --help` through the root command and returns the output.
func captureHelp(t *testing.T, path ...string) string {
	t.Helper()
	// Commands with DisableFlagParsing (e.g. bench) handle --help inside Run,
	// which means PersistentPreRunE mutates output package globals; restore them.
	output.SaveForTest(t)

	root := rootCommand(t)
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(append(append([]string{}, path...), "--help"))
	if err := root.Execute(); err != nil {
		t.Fatalf("help for %v failed: %v", path, err)
	}
	return buf.String()
}

// TestGolden_Help locks the --help output (flag/UX surface) of the root
// command, every top-level command tree, and the high-traffic single commands
// against drift.
func TestGolden_Help(t *testing.T) {
	cases := []struct {
		name string
		path []string
	}{
		// Root
		{"root-help", nil},
		// Command trees
		{"api-help", []string{"api"}},
		{"app-help", []string{"app"}},
		{"bench-help", []string{"bench"}},
		{"build-help", []string{"build"}},
		{"cache-help", []string{"cache"}},
		{"cloud-help", []string{"cloud"}},
		{"config-help", []string{"config"}},
		{"db-help", []string{"db"}},
		{"doc-help", []string{"doc"}},
		{"doctype-help", []string{"doctype"}},
		{"docker-help", []string{"docker"}},
		{"fixtures-help", []string{"fixtures"}},
		{"image-help", []string{"image"}},
		{"log-help", []string{"log"}},
		{"mcp-help", []string{"mcp"}},
		{"remote-help", []string{"remote"}},
		{"scheduler-help", []string{"scheduler"}},
		{"self-help", []string{"self"}},
		{"site-help", []string{"site"}},
		{"user-help", []string{"user"}},
		{"workspace-help", []string{"workspace"}},
		// Key single commands
		{"start-help", []string{"start"}},
		{"sync-help", []string{"sync"}},
		{"status-help", []string{"status"}},
		{"doctor-help", []string{"doctor"}},
		{"new-help", []string{"new"}},
		{"create-help", []string{"create"}},
		{"init-help", []string{"init"}},
		{"run-help", []string{"run"}},
		{"convert-help", []string{"convert"}},
		{"update-help", []string{"update"}},
		{"upgrade-help", []string{"upgrade"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertGolden(t, tc.name, captureHelp(t, tc.path...))
		})
	}
}
