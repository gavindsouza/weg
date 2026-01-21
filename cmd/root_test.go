/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package cmd

import (
	"bytes"
	"os"
	"testing"
)

func TestRootCommand_Setup(t *testing.T) {
	if rootCmd.Use != "weg" {
		t.Errorf("expected Use 'weg', got %s", rootCmd.Use)
	}

	if rootCmd.Short != "Manage Frappe Deployments" {
		t.Errorf("unexpected Short description: %s", rootCmd.Short)
	}

	if !rootCmd.SilenceUsage {
		t.Error("expected SilenceUsage to be true")
	}

	// Test that PersistentPreRunE is set
	if rootCmd.PersistentPreRunE == nil {
		t.Error("expected PersistentPreRunE to be set")
	}
}

func TestRootCommand_GlobalFlags(t *testing.T) {
	flags := []struct {
		name     string
		short    string
		defValue string
	}{
		{"chdir", "C", ""},
		{"verbose", "v", "0"}, // CountVar, not BoolVar
		{"quiet", "q", "false"},
		{"yes", "y", "false"},
		{"config", "", ""},
		{"output", "o", "auto"},
		{"log-level", "", ""},
		{"debug-categories", "", ""},
	}

	for _, f := range flags {
		t.Run(f.name, func(t *testing.T) {
			flag := rootCmd.PersistentFlags().Lookup(f.name)
			if flag == nil {
				t.Errorf("expected flag --%s to be defined", f.name)
				return
			}

			if f.short != "" && flag.Shorthand != f.short {
				t.Errorf("expected shorthand -%s for --%s, got -%s", f.short, f.name, flag.Shorthand)
			}

			if flag.DefValue != f.defValue {
				t.Errorf("expected default %q for --%s, got %q", f.defValue, f.name, flag.DefValue)
			}
		})
	}
}

func TestRootCommand_HasSubcommands(t *testing.T) {
	// Test that key subcommands are registered
	expectedCmds := []string{
		"api",
		"app",
		"build",
		"cache",
		"cloud",
		"config",
		"db",
		"doc",
		"docker",
		"doctype",
		"fixtures",
		"image",
		"log",
		"remote",
		"scheduler",
		"site",
		"user",
		"workspace",
		"version",
		"self",
	}

	subcommands := make(map[string]bool)
	for _, cmd := range rootCmd.Commands() {
		subcommands[cmd.Name()] = true
	}

	for _, expected := range expectedCmds {
		if !subcommands[expected] {
			t.Errorf("expected subcommand %q to be registered", expected)
		}
	}
}

func TestIsVerbose(t *testing.T) {
	// Save original value
	origVerbose := verbose
	defer func() { verbose = origVerbose }()

	verbose = false
	if IsVerbose() {
		t.Error("expected IsVerbose() to return false")
	}

	verbose = true
	if !IsVerbose() {
		t.Error("expected IsVerbose() to return true")
	}
}

func TestIsQuiet(t *testing.T) {
	// Save original value
	origQuiet := quiet
	defer func() { quiet = origQuiet }()

	quiet = false
	if IsQuiet() {
		t.Error("expected IsQuiet() to return false")
	}

	quiet = true
	if !IsQuiet() {
		t.Error("expected IsQuiet() to return true")
	}
}

func TestAssumeYes(t *testing.T) {
	// Save original value
	origYes := yes
	defer func() { yes = origYes }()

	yes = false
	if AssumeYes() {
		t.Error("expected AssumeYes() to return false")
	}

	yes = true
	if !AssumeYes() {
		t.Error("expected AssumeYes() to return true")
	}
}

func TestGetConfigPath(t *testing.T) {
	// Save original value
	origConfigPath := configPath
	defer func() { configPath = origConfigPath }()

	configPath = ""
	if GetConfigPath() != "" {
		t.Error("expected GetConfigPath() to return empty string")
	}

	configPath = "/custom/config.toml"
	if GetConfigPath() != "/custom/config.toml" {
		t.Errorf("expected GetConfigPath() to return '/custom/config.toml', got %s", GetConfigPath())
	}
}

func TestPrintVerbose(t *testing.T) {
	// Save original values
	origVerbose := verbose
	origStdout := os.Stdout
	defer func() {
		verbose = origVerbose
		os.Stdout = origStdout
	}()

	// Create pipe to capture stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Test verbose=false (should not print)
	verbose = false
	PrintVerbose("test message %s", "arg")
	w.Close()

	var buf bytes.Buffer
	buf.ReadFrom(r)
	if buf.String() != "" {
		t.Errorf("expected no output when verbose=false, got: %s", buf.String())
	}

	// Test verbose=true (should print)
	r, w, _ = os.Pipe()
	os.Stdout = w

	verbose = true
	PrintVerbose("test message %s", "arg")
	w.Close()

	buf.Reset()
	buf.ReadFrom(r)
	expected := "test message arg\n"
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}

func TestPrintInfo(t *testing.T) {
	// Save original values
	origQuiet := quiet
	origStdout := os.Stdout
	defer func() {
		quiet = origQuiet
		os.Stdout = origStdout
	}()

	// Test quiet=true (should not print)
	r, w, _ := os.Pipe()
	os.Stdout = w

	quiet = true
	PrintInfo("test message %s", "arg")
	w.Close()

	var buf bytes.Buffer
	buf.ReadFrom(r)
	if buf.String() != "" {
		t.Errorf("expected no output when quiet=true, got: %s", buf.String())
	}

	// Test quiet=false (should print)
	r, w, _ = os.Pipe()
	os.Stdout = w

	quiet = false
	PrintInfo("test message %s", "arg")
	w.Close()

	buf.Reset()
	buf.ReadFrom(r)
	expected := "test message arg\n"
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}

func TestPrintError(t *testing.T) {
	// Save original stderr
	origStderr := os.Stderr
	defer func() { os.Stderr = origStderr }()

	r, w, _ := os.Pipe()
	os.Stderr = w

	PrintError("test error %s", "message")
	w.Close()

	var buf bytes.Buffer
	buf.ReadFrom(r)
	expected := "Error: test error message\n"
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}

func TestGetProjectRoot(t *testing.T) {
	// Save original value
	origProjectRoot := projectRoot
	defer func() { projectRoot = origProjectRoot }()

	projectRoot = ""
	if GetProjectRoot() != "" {
		t.Error("expected empty string")
	}

	projectRoot = "/test/project"
	if GetProjectRoot() != "/test/project" {
		t.Errorf("expected '/test/project', got %s", GetProjectRoot())
	}
}

func TestGetOriginalDir(t *testing.T) {
	// Save original value
	origOriginalDir := originalDir
	defer func() { originalDir = origOriginalDir }()

	originalDir = ""
	if GetOriginalDir() != "" {
		t.Error("expected empty string")
	}

	originalDir = "/original/dir"
	if GetOriginalDir() != "/original/dir" {
		t.Errorf("expected '/original/dir', got %s", GetOriginalDir())
	}
}
