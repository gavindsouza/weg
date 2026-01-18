/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package cmd

import (
	"testing"
)

func TestSelfCommand_Setup(t *testing.T) {
	if selfCmd.Use != "self" {
		t.Errorf("expected Use 'self', got %s", selfCmd.Use)
	}

	if selfCmd.Short != "Manage weg itself and its dependencies" {
		t.Errorf("unexpected Short description: %s", selfCmd.Short)
	}

	// Test subcommands are registered
	subcommands := selfCmd.Commands()
	expectedCmds := map[string]bool{
		"install-tools": false,
		"doctor":        false,
		"update":        false,
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

func TestInstallToolsCommand_Setup(t *testing.T) {
	if installCmd.Use != "install-tools" {
		t.Errorf("expected Use 'install-tools', got %s", installCmd.Use)
	}

	if installCmd.Short != "Install required tools (devbox, direnv, etc)" {
		t.Errorf("unexpected Short description: %s", installCmd.Short)
	}

	if installCmd.Run == nil {
		t.Error("expected Run to be set")
	}
}

func TestSelfDoctorCommand_Setup(t *testing.T) {
	if selfDoctorCmd.Use != "doctor" {
		t.Errorf("expected Use 'doctor', got %s", selfDoctorCmd.Use)
	}

	if selfDoctorCmd.Short != "Check system setup and environment compatibility" {
		t.Errorf("unexpected Short description: %s", selfDoctorCmd.Short)
	}

	if selfDoctorCmd.Run == nil {
		t.Error("expected Run to be set")
	}
}

func TestSelfUpdateCommand_Setup(t *testing.T) {
	if selfUpdateCmd.Use != "update" {
		t.Errorf("expected Use 'update', got %s", selfUpdateCmd.Use)
	}

	if selfUpdateCmd.Short != "Update weg to the latest version" {
		t.Errorf("unexpected Short description: %s", selfUpdateCmd.Short)
	}

	if selfUpdateCmd.Run == nil {
		t.Error("expected Run to be set")
	}
}
