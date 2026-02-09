package tests

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/gavindsouza/weg/cmd/api"
	"github.com/gavindsouza/weg/cmd/app"
	"github.com/gavindsouza/weg/cmd/mcp"
	"github.com/gavindsouza/weg/cmd/site"
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

// captureHelp runs --help on a command and returns the output.
func captureHelp(t *testing.T, cmd *cobra.Command) string {
	t.Helper()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})
	cmd.Execute()
	return buf.String()
}

func TestGolden_SiteHelp(t *testing.T) {
	assertGolden(t, "site-help", captureHelp(t, site.SiteCmd))
}

func TestGolden_AppHelp(t *testing.T) {
	assertGolden(t, "app-help", captureHelp(t, app.AppCmd))
}

func TestGolden_ApiHelp(t *testing.T) {
	assertGolden(t, "api-help", captureHelp(t, api.ApiCmd))
}

func TestGolden_McpHelp(t *testing.T) {
	assertGolden(t, "mcp-help", captureHelp(t, mcp.McpCmd))
}
