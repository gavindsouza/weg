package tools

import (
	"os"
	"path/filepath"
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
