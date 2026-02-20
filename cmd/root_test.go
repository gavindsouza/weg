/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package cmd

import (
	"bytes"
	"testing"

	"github.com/gavindsouza/weg/internal/output"
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
	output.SaveForTest(t)

	var buf bytes.Buffer
	output.Writer = &buf

	// Test normal level (should not print)
	output.Level = output.VerbosityNormal
	PrintVerbose("test message %s", "arg")
	if buf.String() != "" {
		t.Errorf("expected no output at normal verbosity, got: %s", buf.String())
	}

	// Test verbose level (should print)
	buf.Reset()
	output.Level = output.VerbosityVerbose
	PrintVerbose("test message %s", "arg")
	expected := "test message arg\n"
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}

func TestPrintInfo(t *testing.T) {
	output.SaveForTest(t)

	var buf bytes.Buffer
	output.Writer = &buf

	// Test quiet level (should not print)
	output.Level = output.VerbosityQuiet
	PrintInfo("test message %s", "arg")
	if buf.String() != "" {
		t.Errorf("expected no output at quiet verbosity, got: %s", buf.String())
	}

	// Test normal level (should print)
	buf.Reset()
	output.Level = output.VerbosityNormal
	PrintInfo("test message %s", "arg")
	expected := "test message arg\n"
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}

func TestPrintError(t *testing.T) {
	output.SaveForTest(t)

	var buf bytes.Buffer
	output.ErrWriter = &buf

	PrintError("test error %s", "message")
	expected := "✗ test error message\n"
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
