package services

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager("/test/bench")

	if m.BenchPath != "/test/bench" {
		t.Errorf("BenchPath = %q, want /test/bench", m.BenchPath)
	}
	if m.Verbose {
		t.Error("Verbose should be false by default")
	}
	if m.ProcessComposePort != 0 {
		t.Errorf("ProcessComposePort = %d, want 0", m.ProcessComposePort)
	}
	if m.RunID != "" {
		t.Errorf("RunID = %q, want empty", m.RunID)
	}
}

func TestIsDevboxProject(t *testing.T) {
	t.Run("with devbox.json", func(t *testing.T) {
		tmpDir := t.TempDir()
		m := NewManager(tmpDir)

		// Create devbox.json
		devboxPath := filepath.Join(tmpDir, "devbox.json")
		if err := os.WriteFile(devboxPath, []byte("{}"), 0644); err != nil {
			t.Fatalf("Failed to create devbox.json: %v", err)
		}

		if !m.isDevboxProject() {
			t.Error("isDevboxProject should return true when devbox.json exists")
		}
	})

	t.Run("without devbox.json", func(t *testing.T) {
		tmpDir := t.TempDir()
		m := NewManager(tmpDir)

		if m.isDevboxProject() {
			t.Error("isDevboxProject should return false when devbox.json doesn't exist")
		}
	})
}

func TestContainsEnvVar(t *testing.T) {
	tests := []struct {
		name    string
		envData []byte
		pattern string
		want    bool
	}{
		{
			name:    "single matching var",
			envData: []byte("WEG_RUNNER=abc123\x00"),
			pattern: "WEG_RUNNER=abc123",
			want:    true,
		},
		{
			name:    "matching var among multiple",
			envData: []byte("PATH=/usr/bin\x00WEG_RUNNER=abc123\x00HOME=/home/test\x00"),
			pattern: "WEG_RUNNER=abc123",
			want:    true,
		},
		{
			name:    "no match",
			envData: []byte("PATH=/usr/bin\x00HOME=/home/test\x00"),
			pattern: "WEG_RUNNER=abc123",
			want:    false,
		},
		{
			name:    "partial match is not match",
			envData: []byte("WEG_RUNNER=abc\x00"),
			pattern: "WEG_RUNNER=abc123",
			want:    false,
		},
		{
			name:    "empty env data",
			envData: []byte{},
			pattern: "WEG_RUNNER=abc123",
			want:    false,
		},
		{
			name:    "matching at start",
			envData: []byte("WEG_RUNNER=test\x00OTHER=value\x00"),
			pattern: "WEG_RUNNER=test",
			want:    true,
		},
		{
			name:    "matching at end",
			envData: []byte("OTHER=value\x00WEG_RUNNER=test\x00"),
			pattern: "WEG_RUNNER=test",
			want:    true,
		},
		{
			name:    "matching at end without trailing null",
			envData: []byte("OTHER=value\x00WEG_RUNNER=test"),
			pattern: "WEG_RUNNER=test",
			want:    true,
		},
		{
			name:    "different run ID",
			envData: []byte("WEG_RUNNER=xyz789\x00"),
			pattern: "WEG_RUNNER=abc123",
			want:    false,
		},
		{
			name:    "prefix match in longer value",
			envData: []byte("WEG_RUNNER=abc123extra\x00"),
			pattern: "WEG_RUNNER=abc123",
			want:    true, // Prefix match is valid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsEnvVar(tt.envData, tt.pattern)
			if got != tt.want {
				t.Errorf("containsEnvVar(%q, %q) = %v, want %v", tt.envData, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestManagerFields(t *testing.T) {
	m := &Manager{
		BenchPath:          "/custom/path",
		Verbose:            true,
		ProcessComposePort: 8080,
		RunID:              "myrunid",
	}

	if m.BenchPath != "/custom/path" {
		t.Errorf("BenchPath = %q, want /custom/path", m.BenchPath)
	}
	if !m.Verbose {
		t.Error("Verbose should be true")
	}
	if m.ProcessComposePort != 8080 {
		t.Errorf("ProcessComposePort = %d, want 8080", m.ProcessComposePort)
	}
	if m.RunID != "myrunid" {
		t.Errorf("RunID = %q, want myrunid", m.RunID)
	}
}
