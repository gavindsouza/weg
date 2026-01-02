package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("writes new file", func(t *testing.T) {
		path := filepath.Join(tmpDir, "new-file.txt")
		content := []byte("hello world")

		err := AtomicWrite(path, content, 0644)
		if err != nil {
			t.Fatalf("AtomicWrite failed: %v", err)
		}

		// Verify content
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}
		if string(data) != string(content) {
			t.Errorf("content mismatch: got %q, want %q", data, content)
		}

		// Verify no temp file left behind
		if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
			t.Error("temp file should not exist after successful write")
		}
	})

	t.Run("overwrites existing file", func(t *testing.T) {
		path := filepath.Join(tmpDir, "existing-file.txt")

		// Create initial file
		if err := os.WriteFile(path, []byte("old content"), 0644); err != nil {
			t.Fatalf("failed to create initial file: %v", err)
		}

		// Overwrite with atomic write
		newContent := []byte("new content")
		err := AtomicWrite(path, newContent, 0644)
		if err != nil {
			t.Fatalf("AtomicWrite failed: %v", err)
		}

		// Verify new content
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}
		if string(data) != string(newContent) {
			t.Errorf("content mismatch: got %q, want %q", data, newContent)
		}
	})

	t.Run("creates parent directories", func(t *testing.T) {
		path := filepath.Join(tmpDir, "nested", "dir", "file.txt")
		content := []byte("nested content")

		err := AtomicWrite(path, content, 0644)
		if err != nil {
			t.Fatalf("AtomicWrite failed: %v", err)
		}

		// Verify content
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}
		if string(data) != string(content) {
			t.Errorf("content mismatch: got %q, want %q", data, content)
		}
	})

	t.Run("sets correct permissions", func(t *testing.T) {
		path := filepath.Join(tmpDir, "perms-file.txt")
		content := []byte("content")

		err := AtomicWrite(path, content, 0600)
		if err != nil {
			t.Fatalf("AtomicWrite failed: %v", err)
		}

		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("failed to stat file: %v", err)
		}

		// Check permissions (masking with 0777 to ignore OS-specific bits)
		if info.Mode().Perm()&0777 != 0600 {
			t.Errorf("permission mismatch: got %o, want %o", info.Mode().Perm(), 0600)
		}
	})
}

func TestAtomicWriteString(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "string-file.txt")
	content := "string content"

	err := AtomicWriteString(path, content, 0644)
	if err != nil {
		t.Fatalf("AtomicWriteString failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != content {
		t.Errorf("content mismatch: got %q, want %q", data, content)
	}
}

func TestSafeWrite(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("creates backup of existing file", func(t *testing.T) {
		path := filepath.Join(tmpDir, "safe-file.txt")
		backupPath := path + ".bak"

		// Create initial file
		oldContent := []byte("old content")
		if err := os.WriteFile(path, oldContent, 0644); err != nil {
			t.Fatalf("failed to create initial file: %v", err)
		}

		// Safe write with new content
		newContent := []byte("new content")
		err := SafeWrite(path, newContent, 0644)
		if err != nil {
			t.Fatalf("SafeWrite failed: %v", err)
		}

		// Verify new content
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}
		if string(data) != string(newContent) {
			t.Errorf("content mismatch: got %q, want %q", data, newContent)
		}

		// Verify backup exists with old content
		backupData, err := os.ReadFile(backupPath)
		if err != nil {
			t.Fatalf("failed to read backup: %v", err)
		}
		if string(backupData) != string(oldContent) {
			t.Errorf("backup content mismatch: got %q, want %q", backupData, oldContent)
		}
	})

	t.Run("works without existing file", func(t *testing.T) {
		path := filepath.Join(tmpDir, "new-safe-file.txt")
		content := []byte("content")

		err := SafeWrite(path, content, 0644)
		if err != nil {
			t.Fatalf("SafeWrite failed: %v", err)
		}

		// Verify content
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}
		if string(data) != string(content) {
			t.Errorf("content mismatch: got %q, want %q", data, content)
		}

		// Verify no backup created
		if _, err := os.Stat(path + ".bak"); !os.IsNotExist(err) {
			t.Error("backup should not exist for new file")
		}
	})
}
