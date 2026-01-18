package apps

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestIsGitRepo(t *testing.T) {
	// Find git root from current directory
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		t.Skip("not in a git repository")
	}
	projectRoot := string(output[:len(output)-1]) // trim newline

	if !IsGitRepo(projectRoot) {
		t.Error("expected weg project root to be a git repo")
	}

	// Test with non-git directory
	tmpDir := t.TempDir()
	if IsGitRepo(tmpDir) {
		t.Error("expected temp dir to not be a git repo")
	}

	// Test with non-existent path
	if IsGitRepo("/nonexistent/path") {
		t.Error("expected nonexistent path to not be a git repo")
	}
}

func TestFileExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file
	filePath := filepath.Join(tmpDir, "testfile.txt")
	if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Test existing file
	if !fileExists(filePath) {
		t.Error("expected file to exist")
	}

	// Test non-existent file
	if fileExists(filepath.Join(tmpDir, "nonexistent.txt")) {
		t.Error("expected nonexistent file to not exist")
	}

	// Test directory (should return false)
	if fileExists(tmpDir) {
		t.Error("expected directory to return false for fileExists")
	}
}

func TestGetCurrentBranch(t *testing.T) {
	// Skip if git not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	// Find git root
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		t.Skip("not in a git repository")
	}
	projectRoot := string(output[:len(output)-1])

	branch, err := GetCurrentBranch(projectRoot)
	if err != nil {
		t.Fatalf("failed to get branch: %v", err)
	}

	if branch == "" {
		t.Error("expected non-empty branch name")
	}
}

func TestGetCurrentBranchInvalidRepo(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := GetCurrentBranch(tmpDir)
	if err == nil {
		t.Error("expected error for non-git directory")
	}
}

func TestCloneRepoDestinationExists(t *testing.T) {
	// Test both quiet and non-quiet modes
	for _, quiet := range []bool{false, true} {
		tmpDir := t.TempDir()

		err := CloneRepo("https://github.com/test/repo", "", tmpDir, quiet)
		if err == nil {
			t.Errorf("expected error when destination exists (quiet=%v)", quiet)
		}

		if !contains(err.Error(), "already exists") {
			t.Errorf("expected 'already exists' error, got: %v (quiet=%v)", err, quiet)
		}
	}
}

func TestInstallOptions(t *testing.T) {
	opts := InstallOptions{
		BenchPath:      "/path/to/bench",
		AppsDir:        "/path/to/apps",
		FrappeVersion:  "15",
		Verbose:        true,
		PackageManager: "yarn",
	}

	if opts.BenchPath != "/path/to/bench" {
		t.Errorf("unexpected BenchPath: %s", opts.BenchPath)
	}
	if opts.PackageManager != "yarn" {
		t.Errorf("unexpected PackageManager: %s", opts.PackageManager)
	}
}

func TestLinkLocalAppSourceNotExist(t *testing.T) {
	tmpDir := t.TempDir()
	opts := InstallOptions{
		BenchPath: tmpDir,
		AppsDir:   filepath.Join(tmpDir, "apps"),
	}

	err := LinkLocalApp("myapp", "/nonexistent/source", opts)
	if err == nil {
		t.Error("expected error for nonexistent source")
	}

	if !contains(err.Error(), "does not exist") {
		t.Errorf("expected 'does not exist' error, got: %v", err)
	}
}

func TestLinkLocalAppDestinationNotSymlink(t *testing.T) {
	tmpDir := t.TempDir()

	// Create apps directory
	appsDir := filepath.Join(tmpDir, "apps")
	os.MkdirAll(appsDir, 0755)

	// Create a regular directory at destination (not a symlink)
	destDir := filepath.Join(appsDir, "myapp")
	os.MkdirAll(destDir, 0755)

	// Create source directory
	sourceDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(sourceDir, 0755)

	opts := InstallOptions{
		BenchPath: tmpDir,
		AppsDir:   appsDir,
	}

	err := LinkLocalApp("myapp", sourceDir, opts)
	if err == nil {
		t.Error("expected error when destination is not a symlink")
	}

	if !contains(err.Error(), "not a symlink") {
		t.Errorf("expected 'not a symlink' error, got: %v", err)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
