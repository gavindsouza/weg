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
