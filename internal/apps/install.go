package apps

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gavindsouza/weg/tools"
)

// InstallOptions configures app installation
type InstallOptions struct {
	BenchPath      string
	AppsDir        string
	FrappeVersion  string
	Verbose        bool
	PackageManager string // yarn, pnpm, bun
}

// InstallApp clones and installs a Frappe app
func InstallApp(name, url, branch string, opts InstallOptions) error {
	appPath := filepath.Join(opts.AppsDir, name)

	// Clone if not exists
	if !IsGitRepo(appPath) {
		if opts.Verbose {
			fmt.Printf("Cloning %s from %s@%s...\n", name, url, branch)
		}
		if err := CloneRepoQuiet(url, branch, appPath); err != nil {
			return fmt.Errorf("failed to clone %s: %w", name, err)
		}
	}

	// Install Python dependencies
	if err := InstallPythonDeps(appPath, opts); err != nil {
		return fmt.Errorf("failed to install Python deps for %s: %w", name, err)
	}

	// Install Node dependencies if package.json exists
	packageJSON := filepath.Join(appPath, "package.json")
	if _, err := os.Stat(packageJSON); err == nil {
		if err := InstallNodeDeps(appPath, opts); err != nil {
			return fmt.Errorf("failed to install Node deps for %s: %w", name, err)
		}
	}

	return nil
}

// InstallPythonDeps installs Python dependencies using uv
func InstallPythonDeps(appPath string, opts InstallOptions) error {
	// Check if pyproject.toml or setup.py exists
	hasPyproject := fileExists(filepath.Join(appPath, "pyproject.toml"))
	hasSetupPy := fileExists(filepath.Join(appPath, "setup.py"))

	if !hasPyproject && !hasSetupPy {
		// No Python package to install
		return nil
	}

	// Use uv pip install -e for editable install
	cmd := exec.Command("uv", "pip", "install", "-e", appPath)
	cmd.Dir = opts.BenchPath

	if opts.Verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("uv pip install failed: %w", err)
	}

	return nil
}

// InstallNodeDeps installs Node.js dependencies
func InstallNodeDeps(appPath string, opts InstallOptions) error {
	pm := opts.PackageManager
	if pm == "" {
		// Detect from Frappe version
		fv, err := tools.GetFrappeVersion(tools.NormalizeFrappeVersion(opts.FrappeVersion))
		if err == nil {
			pm = fv.PackageManager
		} else {
			pm = "yarn" // default
		}
	}

	var cmd *exec.Cmd
	switch pm {
	case "pnpm":
		cmd = exec.Command("pnpm", "install", "--frozen-lockfile")
	case "bun":
		cmd = exec.Command("bun", "install", "--frozen-lockfile")
	default: // yarn
		cmd = exec.Command("yarn", "install", "--frozen-lockfile")
	}

	cmd.Dir = appPath

	if opts.Verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	// Don't fail if lockfile doesn't exist, just warn
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Try without frozen lockfile
		switch pm {
		case "pnpm":
			cmd = exec.Command("pnpm", "install")
		case "bun":
			cmd = exec.Command("bun", "install")
		default:
			cmd = exec.Command("yarn", "install")
		}
		cmd.Dir = appPath
		if opts.Verbose {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()
		}
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s install failed: %w\n%s", pm, err, string(output))
		}
	}

	return nil
}

// RemoveApp uninstalls and removes an app
func RemoveApp(name string, opts InstallOptions) error {
	appPath := filepath.Join(opts.AppsDir, name)

	// Uninstall from pip
	cmd := exec.Command("uv", "pip", "uninstall", name)
	cmd.Dir = opts.BenchPath
	cmd.Run() // Ignore errors - app might not be installed

	// Remove directory
	if err := os.RemoveAll(appPath); err != nil {
		return fmt.Errorf("failed to remove app directory: %w", err)
	}

	return nil
}

// UpdateApp pulls latest changes and reinstalls dependencies
func UpdateApp(name string, opts InstallOptions) error {
	appPath := filepath.Join(opts.AppsDir, name)

	// Pull latest
	if err := Pull(appPath); err != nil {
		return fmt.Errorf("failed to pull %s: %w", name, err)
	}

	// Reinstall Python deps
	if err := InstallPythonDeps(appPath, opts); err != nil {
		return fmt.Errorf("failed to update Python deps: %w", err)
	}

	// Reinstall Node deps
	packageJSON := filepath.Join(appPath, "package.json")
	if _, err := os.Stat(packageJSON); err == nil {
		if err := InstallNodeDeps(appPath, opts); err != nil {
			return fmt.Errorf("failed to update Node deps: %w", err)
		}
	}

	return nil
}

// LinkLocalApp creates a symlink for local development
func LinkLocalApp(name, sourcePath string, opts InstallOptions) error {
	destPath := filepath.Join(opts.AppsDir, name)

	// Check source exists
	if _, err := os.Stat(sourcePath); err != nil {
		return fmt.Errorf("source path does not exist: %s", sourcePath)
	}

	// Remove existing if it's a symlink
	if info, err := os.Lstat(destPath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			os.Remove(destPath)
		} else {
			return fmt.Errorf("destination exists and is not a symlink: %s", destPath)
		}
	}

	// Create symlink
	absSource, err := filepath.Abs(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	if err := os.Symlink(absSource, destPath); err != nil {
		return fmt.Errorf("failed to create symlink: %w", err)
	}

	// Install dependencies
	if err := InstallPythonDeps(destPath, opts); err != nil {
		return fmt.Errorf("failed to install Python deps: %w", err)
	}

	packageJSON := filepath.Join(destPath, "package.json")
	if _, err := os.Stat(packageJSON); err == nil {
		if err := InstallNodeDeps(destPath, opts); err != nil {
			return fmt.Errorf("failed to install Node deps: %w", err)
		}
	}

	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
