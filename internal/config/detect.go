package config

import (
	"os"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/output"
)

// Context represents the type of directory we're operating in
type Context int

const (
	// ContextFresh is an empty or new directory with no Frappe-related files
	ContextFresh Context = iota
	// ContextApp is a Frappe app directory (has hooks.py)
	ContextApp
	// ContextBench is a traditional bench directory (has apps/ and sites/)
	ContextBench
	// ContextWegApp is a weg-managed app (has pyproject.toml with [tool.weg])
	ContextWegApp
	// ContextWegBench is a weg-managed bench (has weg.toml or .weg/)
	ContextWegBench
)

// String returns a human-readable name for the context
func (c Context) String() string {
	switch c {
	case ContextFresh:
		return "fresh"
	case ContextApp:
		return "frappe-app"
	case ContextBench:
		return "bench"
	case ContextWegApp:
		return "weg-app"
	case ContextWegBench:
		return "weg-bench"
	default:
		return "unknown"
	}
}

// DetectionResult contains the detected context and relevant paths
type DetectionResult struct {
	Context    Context
	Path       string
	AppName    string // Set if ContextApp or ContextWegApp
	BenchPath  string // Set if within a bench
	ConfigPath string // Path to config file (pyproject.toml or weg.toml)
}

// DetectContext analyzes a directory to determine its context
func DetectContext(path string) (*DetectionResult, error) {
	defer output.WithTiming(output.DebugConfig, "DetectContext")()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	result := &DetectionResult{
		Path:    absPath,
		Context: ContextFresh,
	}

	// Check for weg.toml first (highest priority for weg-managed bench)
	if HasWegToml(absPath) {
		result.Context = ContextWegBench
		result.ConfigPath = filepath.Join(absPath, "weg.toml")
		result.BenchPath = absPath
		output.Debugf(output.DebugConfig, "detected context=%s path=%s", result.Context, absPath)
		return result, nil
	}

	// Check for .weg directory (weg-managed environment)
	wegDir := filepath.Join(absPath, ".weg")
	if dirExists(wegDir) {
		// This is likely an app with a hidden weg bench
		if hasHooksPy(absPath) {
			result.Context = ContextWegApp
			result.AppName = filepath.Base(absPath)
			result.BenchPath = wegDir
			if HasWegSection(absPath) {
				result.ConfigPath = filepath.Join(absPath, "pyproject.toml")
			}
			return result, nil
		}
		// Or a weg-managed bench in .weg
		result.Context = ContextWegBench
		result.BenchPath = wegDir
		return result, nil
	}

	// Check for pyproject.toml with [tool.weg] section
	if HasWegSection(absPath) {
		result.Context = ContextWegApp
		result.AppName = filepath.Base(absPath)
		result.BenchPath = filepath.Join(absPath, ".weg")
		result.ConfigPath = filepath.Join(absPath, "pyproject.toml")
		return result, nil
	}

	// Check for hooks.py (Frappe app without weg config)
	if hasHooksPy(absPath) {
		result.Context = ContextApp
		result.AppName = filepath.Base(absPath)
		return result, nil
	}

	// Check for traditional bench structure (apps/ + sites/)
	appsDir := filepath.Join(absPath, "apps")
	sitesDir := filepath.Join(absPath, "sites")
	if dirExists(appsDir) && dirExists(sitesDir) {
		result.Context = ContextBench
		result.BenchPath = absPath
		return result, nil
	}

	// Nothing detected - fresh directory
	return result, nil
}

// FindBenchRoot walks up the directory tree to find a bench root
func FindBenchRoot(startPath string) (string, bool) {
	absPath, err := filepath.Abs(startPath)
	if err != nil {
		return "", false
	}

	current := absPath
	for {
		// Check for weg.toml
		if HasWegToml(current) {
			return current, true
		}

		// Check for traditional bench
		appsDir := filepath.Join(current, "apps")
		sitesDir := filepath.Join(current, "sites")
		if dirExists(appsDir) && dirExists(sitesDir) {
			return current, true
		}

		// Check for .weg directory
		wegDir := filepath.Join(current, ".weg")
		if dirExists(wegDir) {
			return current, true
		}

		// Move up one directory
		parent := filepath.Dir(current)
		if parent == current {
			// Reached root
			return "", false
		}
		current = parent
	}
}

// FindAppRoot walks up the directory tree to find an app root (hooks.py)
func FindAppRoot(startPath string) (string, bool) {
	absPath, err := filepath.Abs(startPath)
	if err != nil {
		return "", false
	}

	current := absPath
	for {
		if hasHooksPy(current) {
			return current, true
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", false
		}
		current = parent
	}
}

// IsInsideBench checks if the current path is inside a bench
func IsInsideBench(path string) bool {
	_, found := FindBenchRoot(path)
	return found
}

// IsInsideApp checks if the current path is inside a Frappe app
func IsInsideApp(path string) bool {
	_, found := FindAppRoot(path)
	return found
}

// hasHooksPy checks if hooks.py exists in the directory or a subdirectory module
func hasHooksPy(path string) bool {
	// Check direct hooks.py
	hooksPath := filepath.Join(path, "hooks.py")
	if _, err := os.Stat(hooksPath); err == nil {
		return true
	}

	// Check for hooks.py in a subdirectory (app module)
	// For app-centric projects, hooks.py is in <project>/<module>/hooks.py
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// Skip hidden directories and common non-app directories
		name := entry.Name()
		if name[0] == '.' || name == "node_modules" || name == "docs" || name == "tests" {
			continue
		}

		moduleHooksPath := filepath.Join(path, name, "hooks.py")
		if _, err := os.Stat(moduleHooksPath); err == nil {
			return true
		}
	}

	return false
}

// dirExists checks if a directory exists
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// IsWegManaged returns true if this is a weg-managed project (app or bench)
func (r *DetectionResult) IsWegManaged() bool {
	return r.Context == ContextWegApp || r.Context == ContextWegBench
}

// ContextDescription returns a user-friendly description of the context
func (r *DetectionResult) ContextDescription() string {
	switch r.Context {
	case ContextFresh:
		return "Empty directory - ready for new project"
	case ContextApp:
		return "Frappe app detected (not weg-managed)"
	case ContextBench:
		return "Traditional bench detected (not weg-managed)"
	case ContextWegApp:
		return "Weg-managed Frappe app"
	case ContextWegBench:
		return "Weg-managed bench"
	default:
		return "Unknown context"
	}
}

// SuggestAction returns a suggested weg command based on context
func (r *DetectionResult) SuggestAction() string {
	switch r.Context {
	case ContextFresh:
		return "Run 'weg new' to create a new Frappe app, or 'weg init' to set up a bench"
	case ContextApp:
		return "Run 'weg init' to add weg configuration to this app"
	case ContextBench:
		return "Run 'weg init' to import this bench into weg management"
	case ContextWegApp:
		return "Run 'weg sync' to ensure environment matches configuration"
	case ContextWegBench:
		return "Run 'weg sync' to ensure environment matches configuration"
	default:
		return ""
	}
}
