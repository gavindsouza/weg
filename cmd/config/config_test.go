/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package config

import (
	"testing"

	"github.com/gavindsouza/weg/internal/config"
)

func TestConfigCommand_Setup(t *testing.T) {
	if ConfigCmd.Use != "config" {
		t.Errorf("expected Use 'config', got %s", ConfigCmd.Use)
	}

	if ConfigCmd.Short != "View and modify weg configuration" {
		t.Errorf("unexpected Short description: %s", ConfigCmd.Short)
	}

	// Test subcommands are registered
	subcommands := ConfigCmd.Commands()
	expectedCmds := map[string]bool{
		"show":      false,
		"get":       false,
		"set":       false,
		"list-apps": false,
	}

	for _, cmd := range subcommands {
		// cmd.Name() returns just the command name without args
		if _, exists := expectedCmds[cmd.Name()]; exists {
			expectedCmds[cmd.Name()] = true
		}
	}

	for name, found := range expectedCmds {
		if !found {
			t.Errorf("expected subcommand %q to be registered", name)
		}
	}
}

func TestShowCommand_Setup(t *testing.T) {
	if showCmd.Use != "show" {
		t.Errorf("expected Use 'show', got %s", showCmd.Use)
	}

	if showCmd.RunE == nil {
		t.Error("expected RunE to be set")
	}

	if !showCmd.SilenceUsage {
		t.Error("expected SilenceUsage to be true")
	}
}

func TestGetCommand_Setup(t *testing.T) {
	if getCmd.Use != "get <key>" {
		t.Errorf("expected Use 'get <key>', got %s", getCmd.Use)
	}

	if getCmd.RunE == nil {
		t.Error("expected RunE to be set")
	}

	if !getCmd.SilenceUsage {
		t.Error("expected SilenceUsage to be true")
	}
}

func TestSetCommand_Setup(t *testing.T) {
	if setCmd.Use != "set <key> <value>" {
		t.Errorf("expected Use 'set <key> <value>', got %s", setCmd.Use)
	}

	if setCmd.RunE == nil {
		t.Error("expected RunE to be set")
	}

	if !setCmd.SilenceUsage {
		t.Error("expected SilenceUsage to be true")
	}
}

func TestListAppsCommand_Setup(t *testing.T) {
	if listAppsCmd.Use != "list-apps" {
		t.Errorf("expected Use 'list-apps', got %s", listAppsCmd.Use)
	}

	if listAppsCmd.RunE == nil {
		t.Error("expected RunE to be set")
	}

	if !listAppsCmd.SilenceUsage {
		t.Error("expected SilenceUsage to be true")
	}
}

func TestGetValueFromBenchConfig(t *testing.T) {
	cfg := &config.BenchConfig{
		Frappe: config.FrappeSettings{
			Version:  "15",
			Database: "mariadb",
		},
		Bench: config.BenchSettings{
			Name: "test-bench",
		},
		Apps: map[string]config.AppSettings{
			"erpnext": {
				URL:    "https://github.com/frappe/erpnext",
				Branch: "version-15",
			},
			"hrms": {
				Path: "/local/path/hrms",
			},
		},
	}

	tests := []struct {
		name     string
		parts    []string
		expected string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "frappe.version",
			parts:    []string{"frappe", "version"},
			expected: "15",
			wantErr:  false,
		},
		{
			name:     "frappe.database",
			parts:    []string{"frappe", "database"},
			expected: "mariadb",
			wantErr:  false,
		},
		{
			name:     "bench.name",
			parts:    []string{"bench", "name"},
			expected: "test-bench",
			wantErr:  false,
		},
		{
			name:     "apps.erpnext.url",
			parts:    []string{"apps", "erpnext", "url"},
			expected: "https://github.com/frappe/erpnext",
			wantErr:  false,
		},
		{
			name:     "apps.erpnext.branch",
			parts:    []string{"apps", "erpnext", "branch"},
			expected: "version-15",
			wantErr:  false,
		},
		{
			name:     "apps.hrms.path",
			parts:    []string{"apps", "hrms", "path"},
			expected: "/local/path/hrms",
			wantErr:  false,
		},
		{
			name:    "empty key",
			parts:   []string{},
			wantErr: true,
			errMsg:  "empty key",
		},
		{
			name:    "missing frappe key",
			parts:   []string{"frappe"},
			wantErr: true,
			errMsg:  "missing frappe key",
		},
		{
			name:    "missing bench key",
			parts:   []string{"bench"},
			wantErr: true,
			errMsg:  "missing bench key",
		},
		{
			name:    "apps missing parts",
			parts:   []string{"apps", "erpnext"},
			wantErr: true,
			errMsg:  "usage: apps.<name>.<key>",
		},
		{
			name:    "app not found",
			parts:   []string{"apps", "nonexistent", "url"},
			wantErr: true,
			errMsg:  "app not found: nonexistent",
		},
		{
			name:    "unknown key",
			parts:   []string{"unknown", "key"},
			wantErr: true,
			errMsg:  "key not found: unknown.key",
		},
		{
			name:    "unknown frappe subkey",
			parts:   []string{"frappe", "unknown"},
			wantErr: true,
			errMsg:  "key not found: frappe.unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := getValueFromBenchConfig(cfg, tt.parts)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
					return
				}
				if err.Error() != tt.errMsg {
					t.Errorf("expected error %q, got %q", tt.errMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
