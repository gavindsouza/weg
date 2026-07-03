package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/testutil"
)

// Bare `weg build` is documented as equivalent to `weg build assets`: both
// accept at most one app argument and expose the same flags.
func TestBuildCommand_ArgAndFlagParity(t *testing.T) {
	if err := BuildCmd.Args(BuildCmd, []string{"one", "two"}); err == nil {
		t.Error("build: expected error for more than one app arg")
	}
	if err := BuildCmd.Args(BuildCmd, []string{"myapp"}); err != nil {
		t.Errorf("build: one app arg should be valid: %v", err)
	}
	if err := assetsCmd.Args(assetsCmd, []string{"one", "two"}); err == nil {
		t.Error("assets: expected error for more than one app arg")
	}

	for _, flag := range []string{"site", "hard", "production"} {
		if BuildCmd.Flags().Lookup(flag) == nil {
			t.Errorf("build: missing --%s flag (parity with assets)", flag)
		}
		if assetsCmd.Flags().Lookup(flag) == nil {
			t.Errorf("assets: missing --%s flag", flag)
		}
	}
}

// runAssets must fail fast with a project error outside a weg project,
// before ever attempting to invoke devbox.
func TestRunAssets_NotAWegProject(t *testing.T) {
	t.Chdir(t.TempDir())
	output.CaptureForTest(t)

	err := runAssets(assetsCmd, nil)
	if err == nil {
		t.Fatal("expected error outside a weg project")
	}
}

func TestResolveContext(t *testing.T) {
	t.Run("not a weg project", func(t *testing.T) {
		t.Chdir(t.TempDir())
		if _, _, err := resolveContext(""); err == nil {
			t.Fatal("expected error outside a weg project")
		}
	})

	t.Run("bench without site", func(t *testing.T) {
		bench := testutil.NewBench(t).WithApp("frappe").Build()
		t.Chdir(bench)

		_, _, err := resolveContext("")
		if err == nil {
			t.Fatal("expected error when no site is specified or discoverable")
		}
		if !strings.Contains(err.Error(), "no site specified") {
			t.Errorf("expected no-site usage error, got: %v", err)
		}
	})

	t.Run("explicit site wins", func(t *testing.T) {
		bench := testutil.NewBench(t).WithApp("frappe").WithSite("a.localhost").Build()
		t.Chdir(bench)

		benchPath, site, err := resolveContext("b.localhost")
		if err != nil {
			t.Fatalf("resolveContext: %v", err)
		}
		if site != "b.localhost" {
			t.Errorf("site = %q, want explicit b.localhost", site)
		}
		if benchPath == "" {
			t.Error("bench path empty")
		}
	})

	t.Run("falls back to currentsite.txt", func(t *testing.T) {
		bench := testutil.NewBench(t).WithApp("frappe").WithSite("a.localhost").Build()
		current := filepath.Join(bench, "sites", "currentsite.txt")
		if err := os.WriteFile(current, []byte("a.localhost\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Chdir(bench)

		_, site, err := resolveContext("")
		if err != nil {
			t.Fatalf("resolveContext: %v", err)
		}
		if site != "a.localhost" {
			t.Errorf("site = %q, want a.localhost from currentsite.txt", site)
		}
	})
}
