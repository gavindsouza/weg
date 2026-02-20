package apps

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/fsutil"
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
		if err := CloneRepo(url, branch, appPath, true); err != nil {
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
	hasPyproject := fsutil.FileExists(filepath.Join(appPath, "pyproject.toml"))
	hasSetupPy := fsutil.FileExists(filepath.Join(appPath, "setup.py"))

	if !hasPyproject && !hasSetupPy {
		// No Python package to install
		return nil
	}

	runner := tools.NewRunnerWithOptions(opts.BenchPath, opts.Verbose)
	if err := runner.Run("uv", "pip", "install", "-e", appPath); err != nil {
		return fmt.Errorf("failed to install Python dependencies: %w", err)
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

	// Use bench path for devbox detection, run commands in app path
	runner := tools.NewRunnerWithOptions(opts.BenchPath, opts.Verbose)

	// Try with frozen lockfile first, fall back to without
	var err error
	switch pm {
	case "pnpm":
		err = runner.RunInDir(appPath, "pnpm", "install", "--frozen-lockfile")
		if err != nil {
			err = runner.RunInDir(appPath, "pnpm", "install")
		}
	case "bun":
		err = runner.RunInDir(appPath, "bun", "install", "--frozen-lockfile")
		if err != nil {
			err = runner.RunInDir(appPath, "bun", "install")
		}
	default: // yarn
		err = runner.RunInDir(appPath, "yarn", "install", "--frozen-lockfile")
		if err != nil {
			err = runner.RunInDir(appPath, "yarn", "install")
		}
	}

	if err != nil {
		return fmt.Errorf("failed to install Node.js dependencies with %s: %w", pm, err)
	}
	return nil
}

// RemoveApp uninstalls an app from all sites and removes it from the bench
func RemoveApp(name string, opts InstallOptions) error {
	appPath := filepath.Join(opts.AppsDir, name)

	// First, uninstall the app from all sites that have it installed
	sitesDir := filepath.Join(opts.BenchPath, "sites")
	if entries, err := os.ReadDir(sitesDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			siteName := entry.Name()
			// Skip non-site directories
			if siteName == "assets" || siteName[0] == '.' {
				continue
			}
			// Check if this is actually a site (has site_config.json)
			siteConfigPath := filepath.Join(sitesDir, siteName, "site_config.json")
			if _, err := os.Stat(siteConfigPath); os.IsNotExist(err) {
				continue
			}

			// Try to uninstall the app from this site
			// This will fail gracefully if the app isn't installed on this site
			if opts.Verbose {
				fmt.Printf("Uninstalling %s from site %s...\n", name, siteName)
			}
			if err := UninstallAppFromSite(siteName, name, opts); err != nil {
				// Log but don't fail - app might not be installed on this site
				if opts.Verbose {
					fmt.Printf("  Note: %s may not be installed on %s: %v\n", name, siteName, err)
				}
			}
		}
	}

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

// InstallAppOnSite installs an app on a Frappe site
func InstallAppOnSite(siteName, appName string, opts InstallOptions) error {
	sitesDir := filepath.Join(opts.BenchPath, "sites")
	pythonPath := filepath.Join(opts.BenchPath, "env", "bin", "python")

	cmd := exec.Command("devbox", "run", "-c", opts.BenchPath, "--",
		pythonPath, "-m", "frappe.utils.bench_helper",
		"frappe", "--site", siteName, "install-app", appName)
	cmd.Dir = sitesDir
	if opts.Verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	return cmd.Run()
}

// UninstallAppFromSite uninstalls an app from a Frappe site
func UninstallAppFromSite(siteName, appName string, opts InstallOptions) error {
	sitesDir := filepath.Join(opts.BenchPath, "sites")
	pythonPath := filepath.Join(opts.BenchPath, "env", "bin", "python")

	cmd := exec.Command("devbox", "run", "-c", opts.BenchPath, "--",
		pythonPath, "-m", "frappe.utils.bench_helper",
		"frappe", "--site", siteName, "uninstall-app", appName, "--yes")
	cmd.Dir = sitesDir
	if opts.Verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	return cmd.Run()
}
