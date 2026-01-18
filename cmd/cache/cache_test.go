/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package cache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCacheCommand_Setup(t *testing.T) {
	if CacheCmd.Use != "cache" {
		t.Errorf("expected Use 'cache', got %s", CacheCmd.Use)
	}

	if CacheCmd.Short != "Manage Frappe cache" {
		t.Errorf("unexpected Short description: %s", CacheCmd.Short)
	}

	// Test subcommands are registered
	subcommands := CacheCmd.Commands()
	found := false
	for _, cmd := range subcommands {
		if cmd.Name() == "clear" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected 'clear' subcommand to be registered")
	}
}

func TestClearCommand_Setup(t *testing.T) {
	if clearCmd.Use != "clear" {
		t.Errorf("expected Use 'clear', got %s", clearCmd.Use)
	}

	if clearCmd.RunE == nil {
		t.Error("expected RunE to be set")
	}

	// Test flags are defined
	siteFlag := clearCmd.Flags().Lookup("site")
	if siteFlag == nil {
		t.Error("expected --site flag to be defined")
	}

	allFlag := clearCmd.Flags().Lookup("all")
	if allFlag == nil {
		t.Error("expected --all flag to be defined")
	}

	if allFlag.DefValue != "false" {
		t.Errorf("expected --all default false, got %s", allFlag.DefValue)
	}
}

func TestClearPycache(t *testing.T) {
	// Create temp directory structure
	tmpDir, err := os.MkdirTemp("", "weg-test-pycache-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create directory structure with __pycache__ and .pyc files
	pycacheDir := filepath.Join(tmpDir, "app", "__pycache__")
	if err := os.MkdirAll(pycacheDir, 0755); err != nil {
		t.Fatalf("failed to create pycache dir: %v", err)
	}

	// Create .pyc files inside __pycache__
	pycFile1 := filepath.Join(pycacheDir, "module.cpython-39.pyc")
	if err := os.WriteFile(pycFile1, []byte("pyc content"), 0644); err != nil {
		t.Fatalf("failed to create pyc file: %v", err)
	}

	// Create .pyc file outside __pycache__
	pycFile2 := filepath.Join(tmpDir, "app", "old.pyc")
	if err := os.WriteFile(pycFile2, []byte("old pyc"), 0644); err != nil {
		t.Fatalf("failed to create pyc file: %v", err)
	}

	// Create a regular Python file that should NOT be deleted
	pyFile := filepath.Join(tmpDir, "app", "module.py")
	if err := os.WriteFile(pyFile, []byte("python code"), 0644); err != nil {
		t.Fatalf("failed to create py file: %v", err)
	}

	// Run clearPycache
	clearPycache(tmpDir)

	// Verify __pycache__ directory is removed
	if _, err := os.Stat(pycacheDir); !os.IsNotExist(err) {
		t.Error("expected __pycache__ directory to be removed")
	}

	// Verify standalone .pyc file is removed
	if _, err := os.Stat(pycFile2); !os.IsNotExist(err) {
		t.Error("expected .pyc file to be removed")
	}

	// Verify .py file still exists
	if _, err := os.Stat(pyFile); os.IsNotExist(err) {
		t.Error("expected .py file to still exist")
	}
}

func TestClearPycache_NestedDirectories(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "weg-test-pycache-nested-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create nested __pycache__ directories
	dirs := []string{
		filepath.Join(tmpDir, "app1", "__pycache__"),
		filepath.Join(tmpDir, "app1", "sub", "__pycache__"),
		filepath.Join(tmpDir, "app2", "__pycache__"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
		// Create a file in each pycache
		if err := os.WriteFile(filepath.Join(dir, "test.pyc"), []byte("pyc"), 0644); err != nil {
			t.Fatalf("failed to create pyc file: %v", err)
		}
	}

	clearPycache(tmpDir)

	// Verify all __pycache__ directories are removed
	for _, dir := range dirs {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Errorf("expected %s to be removed", dir)
		}
	}

	// Verify parent directories still exist
	parentDirs := []string{
		filepath.Join(tmpDir, "app1"),
		filepath.Join(tmpDir, "app1", "sub"),
		filepath.Join(tmpDir, "app2"),
	}

	for _, dir := range parentDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("expected parent directory %s to still exist", dir)
		}
	}
}

func TestClearPycache_EmptyDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "weg-test-pycache-empty-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Should not panic or error on empty directory
	clearPycache(tmpDir)
}

func TestClearPycache_NonexistentDirectory(t *testing.T) {
	// Should not panic on nonexistent directory
	clearPycache("/nonexistent/path/that/does/not/exist")
}
