// Package fsutil provides filesystem utilities with safety guarantees.
package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// AtomicWrite writes data to a file atomically.
// It writes to a temporary file first, then renames it to the target path.
// This ensures the file is never in a partially written state.
func AtomicWrite(path string, data []byte, perm os.FileMode) error {
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Write to temporary file in the same directory
	// (same directory ensures same filesystem for atomic rename)
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, perm); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath) // Clean up on failure
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// AtomicWriteString writes a string to a file atomically.
func AtomicWriteString(path string, content string, perm os.FileMode) error {
	return AtomicWrite(path, []byte(content), perm)
}

// SafeWrite writes data to a file, creating a backup first if the file exists.
// Use this for critical config files where you want recovery options.
func SafeWrite(path string, data []byte, perm os.FileMode) error {
	// Check if file exists
	if _, err := os.Stat(path); err == nil {
		// Create backup
		backupPath := path + ".bak"
		if err := copyFile(path, backupPath); err != nil {
			return fmt.Errorf("failed to create backup: %w", err)
		}
	}

	// Write atomically
	if err := AtomicWrite(path, data, perm); err != nil {
		return err
	}

	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.WriteFile(dst, data, info.Mode())
}
