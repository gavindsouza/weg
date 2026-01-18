/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package app

import (
	"testing"
)

func TestAppCommand_Setup(t *testing.T) {
	if AppCmd.Use != "app" {
		t.Errorf("expected Use 'app', got %s", AppCmd.Use)
	}

	if AppCmd.Short != "Manage Frappe apps" {
		t.Errorf("unexpected Short description: %s", AppCmd.Short)
	}

	// Test subcommands are registered
	subcommands := AppCmd.Commands()
	expectedCmds := map[string]bool{
		"list":    false,
		"get":     false,
		"remove":  false,
		"switch":  false,
		"exclude": false,
		"include": false,
	}

	for _, cmd := range subcommands {
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

func TestListCommand_Setup(t *testing.T) {
	if listCmd.Use != "list" {
		t.Errorf("expected Use 'list', got %s", listCmd.Use)
	}

	if listCmd.RunE == nil {
		t.Error("expected RunE to be set")
	}
}

func TestGetCommand_Setup(t *testing.T) {
	if getCmd.Name() != "get" {
		t.Errorf("expected Name 'get', got %s", getCmd.Name())
	}

	if getCmd.RunE == nil {
		t.Error("expected RunE to be set")
	}
}

func TestRemoveCommand_Setup(t *testing.T) {
	if removeCmd.Name() != "remove" {
		t.Errorf("expected Name 'remove', got %s", removeCmd.Name())
	}

	if removeCmd.RunE == nil {
		t.Error("expected RunE to be set")
	}
}

func TestSwitchCommand_Setup(t *testing.T) {
	if switchCmd.Name() != "switch" {
		t.Errorf("expected Name 'switch', got %s", switchCmd.Name())
	}

	if switchCmd.RunE == nil {
		t.Error("expected RunE to be set")
	}
}

func TestExcludeCommand_Setup(t *testing.T) {
	if excludeCmd.Name() != "exclude" {
		t.Errorf("expected Name 'exclude', got %s", excludeCmd.Name())
	}

	if excludeCmd.RunE == nil {
		t.Error("expected RunE to be set")
	}
}

func TestParseAppSpec(t *testing.T) {
	tests := []struct {
		name         string
		spec         string
		expectedURL  string
		expectedName string
	}{
		{
			name:         "full https url",
			spec:         "https://github.com/frappe/erpnext",
			expectedURL:  "https://github.com/frappe/erpnext",
			expectedName: "erpnext",
		},
		{
			name:         "full https url with .git",
			spec:         "https://github.com/frappe/erpnext.git",
			expectedURL:  "https://github.com/frappe/erpnext.git",
			expectedName: "erpnext",
		},
		{
			name:         "short github reference",
			spec:         "frappe/erpnext",
			expectedURL:  "https://github.com/frappe/erpnext",
			expectedName: "erpnext",
		},
		{
			name:         "git ssh url",
			spec:         "git@github.com:frappe/erpnext.git",
			expectedURL:  "git@github.com:frappe/erpnext.git",
			expectedName: "erpnext",
		},
		{
			name:         "just app name",
			spec:         "erpnext",
			expectedURL:  "",
			expectedName: "erpnext",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, name := parseAppSpec(tt.spec)
			if url != tt.expectedURL {
				t.Errorf("parseAppSpec(%q) url = %q, expected %q", tt.spec, url, tt.expectedURL)
			}
			if name != tt.expectedName {
				t.Errorf("parseAppSpec(%q) name = %q, expected %q", tt.spec, name, tt.expectedName)
			}
		})
	}
}

func TestExtractAppName(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "github https",
			url:      "https://github.com/frappe/erpnext",
			expected: "erpnext",
		},
		{
			name:     "github https with .git",
			url:      "https://github.com/frappe/erpnext.git",
			expected: "erpnext",
		},
		{
			name:     "gitlab",
			url:      "https://gitlab.com/user/myapp",
			expected: "myapp",
		},
		{
			name:     "simple name",
			url:      "erpnext",
			expected: "erpnext",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractAppName(tt.url)
			if result != tt.expected {
				t.Errorf("extractAppName(%q) = %q, expected %q", tt.url, result, tt.expected)
			}
		})
	}
}
