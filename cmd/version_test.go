/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestVersionCommand_Setup(t *testing.T) {
	// Test command is properly configured
	if versionCmd.Use != "version" {
		t.Errorf("expected Use 'version', got %s", versionCmd.Use)
	}

	if versionCmd.Short != "Show version information" {
		t.Errorf("unexpected Short description: %s", versionCmd.Short)
	}

	// Test --apps flag exists
	flag := versionCmd.Flags().Lookup("apps")
	if flag == nil {
		t.Error("expected --apps flag to be defined")
	}

	if flag.DefValue != "false" {
		t.Errorf("expected --apps default false, got %s", flag.DefValue)
	}
}

func TestRunVersionCmd_Basic(t *testing.T) {
	// Save original values
	origVersion := Version
	origCommit := Commit
	origBuildDate := BuildDate
	origShowApps := showApps
	defer func() {
		Version = origVersion
		Commit = origCommit
		BuildDate = origBuildDate
		showApps = origShowApps
	}()

	// Set test values
	Version = "1.2.3"
	Commit = "abc123"
	BuildDate = "2025-01-01"
	showApps = false

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runVersionCmd(versionCmd, nil)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !bytes.Contains([]byte(output), []byte("weg 1.2.3")) {
		t.Errorf("expected output to contain version, got: %s", output)
	}

	if !bytes.Contains([]byte(output), []byte("commit: abc123")) {
		t.Errorf("expected output to contain commit, got: %s", output)
	}

	if !bytes.Contains([]byte(output), []byte("built:  2025-01-01")) {
		t.Errorf("expected output to contain build date, got: %s", output)
	}
}

func TestRunVersionCmd_UnknownCommit(t *testing.T) {
	origVersion := Version
	origCommit := Commit
	origBuildDate := BuildDate
	origShowApps := showApps
	defer func() {
		Version = origVersion
		Commit = origCommit
		BuildDate = origBuildDate
		showApps = origShowApps
	}()

	Version = "dev"
	Commit = "unknown"
	BuildDate = "unknown"
	showApps = false

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runVersionCmd(versionCmd, nil)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !bytes.Contains([]byte(output), []byte("weg dev")) {
		t.Errorf("expected output to contain 'weg dev', got: %s", output)
	}

	// Should not contain commit or built lines when unknown
	if bytes.Contains([]byte(output), []byte("commit:")) {
		t.Errorf("should not show commit when unknown, got: %s", output)
	}

	if bytes.Contains([]byte(output), []byte("built:")) {
		t.Errorf("should not show built when unknown, got: %s", output)
	}
}

func TestIndexOf(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected int
	}{
		{
			name:     "found at start",
			s:        "hello world",
			substr:   "hello",
			expected: 0,
		},
		{
			name:     "found in middle",
			s:        "hello world",
			substr:   "world",
			expected: 6,
		},
		{
			name:     "not found",
			s:        "hello world",
			substr:   "foo",
			expected: -1,
		},
		{
			name:     "empty string",
			s:        "",
			substr:   "foo",
			expected: -1,
		},
		{
			name:     "empty substr",
			s:        "hello",
			substr:   "",
			expected: 0,
		},
		{
			name:     "version pattern",
			s:        `__version__ = "1.2.3"`,
			substr:   `__version__ = "`,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := indexOf(tt.s, tt.substr)
			if result != tt.expected {
				t.Errorf("indexOf(%q, %q) = %d, expected %d", tt.s, tt.substr, result, tt.expected)
			}
		})
	}
}

func TestGetAppVersion(t *testing.T) {
	tests := []struct {
		name        string
		setupFiles  map[string]string
		appName     string
		expected    string
		description string
	}{
		{
			name: "python double quote version",
			setupFiles: map[string]string{
				"apps/myapp/myapp/__init__.py": `__version__ = "1.2.3"`,
			},
			appName:     "myapp",
			expected:    "1.2.3",
			description: "should parse __version__ with double quotes",
		},
		{
			name: "python single quote version",
			setupFiles: map[string]string{
				"apps/myapp/myapp/__init__.py": `__version__ = '2.0.0'`,
			},
			appName:     "myapp",
			expected:    "2.0.0",
			description: "should parse __version__ with single quotes",
		},
		{
			name: "python no space version",
			setupFiles: map[string]string{
				"apps/myapp/myapp/__init__.py": `__version__="3.1.4"`,
			},
			appName:     "myapp",
			expected:    "3.1.4",
			description: "should parse __version__ without spaces",
		},
		{
			name: "package.json version",
			setupFiles: map[string]string{
				"apps/jsapp/package.json": `{"name": "jsapp", "version": "4.5.6"}`,
			},
			appName:     "jsapp",
			expected:    "4.5.6",
			description: "should parse version from package.json",
		},
		{
			name:        "no version files",
			setupFiles:  map[string]string{},
			appName:     "noapp",
			expected:    "",
			description: "should return empty string when no version found",
		},
		{
			name: "prefer python over package.json",
			setupFiles: map[string]string{
				"apps/both/both/__init__.py": `__version__ = "1.0.0"`,
				"apps/both/package.json":     `{"version": "2.0.0"}`,
			},
			appName:     "both",
			expected:    "1.0.0",
			description: "should prefer Python version over package.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory
			tmpDir, err := os.MkdirTemp("", "weg-test-*")
			if err != nil {
				t.Fatalf("failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			// Setup files
			for path, content := range tt.setupFiles {
				fullPath := filepath.Join(tmpDir, path)
				if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
					t.Fatalf("failed to create dir: %v", err)
				}
				if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
					t.Fatalf("failed to write file: %v", err)
				}
			}

			result := getAppVersion(tmpDir, tt.appName)
			if result != tt.expected {
				t.Errorf("%s: getAppVersion() = %q, expected %q", tt.description, result, tt.expected)
			}
		})
	}
}
