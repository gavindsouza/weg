package tools

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewRunner(t *testing.T) {
	tmpDir := t.TempDir()

	// Without devbox
	r := NewRunner(tmpDir)
	if r.Dir != tmpDir {
		t.Errorf("Dir = %v, want %v", r.Dir, tmpDir)
	}
	if r.UsesDevbox() {
		t.Error("expected UsesDevbox to be false without devbox.json")
	}

	// With devbox
	devboxFile := filepath.Join(tmpDir, "devbox.json")
	if err := os.WriteFile(devboxFile, []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to create devbox.json: %v", err)
	}

	r = NewRunner(tmpDir)
	if !r.UsesDevbox() {
		t.Error("expected UsesDevbox to be true with devbox.json")
	}
}

func TestNewRunnerWithOptions(t *testing.T) {
	tmpDir := t.TempDir()

	r := NewRunnerWithOptions(tmpDir, true)
	if !r.Verbose {
		t.Error("expected Verbose to be true")
	}

	r = NewRunnerWithOptions(tmpDir, false)
	if r.Verbose {
		t.Error("expected Verbose to be false")
	}
}

func TestEnsureDevbox(t *testing.T) {
	tmpDir := t.TempDir()

	// Without devbox.json
	err := EnsureDevbox(tmpDir)
	if err == nil {
		t.Error("expected error when devbox.json is missing")
	}

	// With devbox.json
	devboxFile := filepath.Join(tmpDir, "devbox.json")
	if err := os.WriteFile(devboxFile, []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to create devbox.json: %v", err)
	}

	err = EnsureDevbox(tmpDir)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunError(t *testing.T) {
	err := &RunError{
		Command: "test",
		Args:    []string{"arg1", "arg2"},
		Output:  []byte("error output"),
		Err:     os.ErrNotExist,
	}

	errMsg := err.Error()
	if errMsg == "" {
		t.Error("expected non-empty error message")
	}

	unwrapped := err.Unwrap()
	if unwrapped != os.ErrNotExist {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, os.ErrNotExist)
	}
}

func TestRunnerDetectDevbox(t *testing.T) {
	tests := []struct {
		name       string
		setupFunc  func(dir string) error
		wantDevbox bool
	}{
		{
			name:       "no devbox",
			setupFunc:  func(dir string) error { return nil },
			wantDevbox: false,
		},
		{
			name: "with devbox.json",
			setupFunc: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "devbox.json"), []byte("{}"), 0644)
			},
			wantDevbox: true,
		},
		{
			name: "devbox.json is directory",
			setupFunc: func(dir string) error {
				return os.Mkdir(filepath.Join(dir, "devbox.json"), 0755)
			},
			wantDevbox: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			if err := tt.setupFunc(tmpDir); err != nil {
				t.Fatalf("setup failed: %v", err)
			}

			r := NewRunner(tmpDir)
			if r.UsesDevbox() != tt.wantDevbox {
				t.Errorf("UsesDevbox() = %v, want %v", r.UsesDevbox(), tt.wantDevbox)
			}
		})
	}
}

func TestFrappeCmdRunsFromSites(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not available")
	}
	if strings.ContainsAny(chdirThenBenchHelper, "$\\`*?[") || strings.Contains(chdirThenBenchHelper, "  ") {
		t.Fatal("script must survive devbox run's double-quote + unquoted eval")
	}

	// Stub frappe.utils.bench_helper that reports its cwd and argv.
	stub := t.TempDir()
	pkg := filepath.Join(stub, "frappe", "utils")
	os.MkdirAll(pkg, 0755)
	os.WriteFile(filepath.Join(stub, "frappe", "__init__.py"), nil, 0644)
	os.WriteFile(filepath.Join(pkg, "__init__.py"), nil, 0644)
	os.WriteFile(filepath.Join(pkg, "bench_helper.py"), []byte("import os, sys\nif __name__ == '__main__':\n    print(os.getcwd(), sys.argv[1:])\n"), 0644)

	sites := filepath.Join(t.TempDir(), "sites dir")
	os.MkdirAll(sites, 0755)

	// devbox run starts in the bench root; the script must still land in sites.
	argv, _ := json.Marshal([]string{sites, "frappe", "--admin-password=pa$$  w`x`", "build"})
	cmd := exec.Command(python, "-c", chdirThenBenchHelper)
	cmd.Dir = stub
	cmd.Env = append(os.Environ(), "PYTHONPATH="+stub, frappeArgvEnv+"="+string(argv))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if want := sites + " ['frappe', '--admin-password=pa$$  w`x`', 'build']"; strings.TrimSpace(string(out)) != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}
