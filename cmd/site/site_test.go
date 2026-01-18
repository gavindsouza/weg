/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package site

import (
	"testing"
)

func TestSiteCommand_Setup(t *testing.T) {
	if SiteCmd.Use != "site" {
		t.Errorf("expected Use 'site', got %s", SiteCmd.Use)
	}

	if SiteCmd.Short != "Manage Frappe sites" {
		t.Errorf("unexpected Short description: %s", SiteCmd.Short)
	}

	// Test subcommands are registered
	subcommands := SiteCmd.Commands()
	expectedCmds := map[string]bool{
		"list":    false,
		"new":     false,
		"drop":    false,
		"use":     false,
		"install": false,
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

func TestNewCommand_Setup(t *testing.T) {
	if newCmd.Name() != "new" {
		t.Errorf("expected Name 'new', got %s", newCmd.Name())
	}

	if newCmd.RunE == nil {
		t.Error("expected RunE to be set")
	}
}

func TestDropCommand_Setup(t *testing.T) {
	if dropCmd.Name() != "drop" {
		t.Errorf("expected Name 'drop', got %s", dropCmd.Name())
	}

	if dropCmd.RunE == nil {
		t.Error("expected RunE to be set")
	}
}

func TestUseCommand_Setup(t *testing.T) {
	if useCmd.Name() != "use" {
		t.Errorf("expected Name 'use', got %s", useCmd.Name())
	}

	if useCmd.RunE == nil {
		t.Error("expected RunE to be set")
	}
}

func TestInstallCommand_Setup(t *testing.T) {
	if installCmd.Name() != "install" {
		t.Errorf("expected Name 'install', got %s", installCmd.Name())
	}

	if installCmd.RunE == nil {
		t.Error("expected RunE to be set")
	}
}
