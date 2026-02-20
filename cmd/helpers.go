package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/state"
)

// BenchContext holds resolved bench context information
type BenchContext struct {
	AbsPath   string                  // Absolute path where command was run
	BenchPath string                  // Path to the bench directory
	Context   config.Context          // Detected context type
	Result    *config.DetectionResult // Full detection result
}

// ResolveBenchPath detects context and returns the bench path.
// Returns an error if not in a weg-managed project.
func ResolveBenchPath() (*BenchContext, error) {
	return ResolveBenchPathFrom(".")
}

// ResolveBenchPathFrom detects context from the given path and returns the bench path.
// Returns an error if not in a weg-managed project.
func ResolveBenchPathFrom(path string) (*BenchContext, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	result, err := config.DetectContext(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to detect context: %w", err)
	}

	if !result.IsWegManaged() {
		return nil, errors.NotInProject(absPath)
	}

	return &BenchContext{
		AbsPath:   absPath,
		BenchPath: result.BenchPath,
		Context:   result.Context,
		Result:    result,
	}, nil
}

// ResolveDefaultSite returns the default site for the project.
// It first checks the state, then falls back to config.
func ResolveDefaultSite(absPath string) string {
	// Try state first
	st, err := state.Load(absPath)
	if err == nil {
		if site := st.GetDefaultSite(); site != "" {
			return site
		}
	}
	return ""
}

// ResolveDefaultSiteWithFallback returns the default site, falling back to
// finding any site in the sites directory.
func ResolveDefaultSiteWithFallback(benchPath string) string {
	// Try state first using bench path
	st, err := state.Load(benchPath)
	if err == nil {
		if site := st.GetDefaultSite(); site != "" {
			return site
		}
	}

	// Fallback: look for any site directory
	sitesDir := filepath.Join(benchPath, "sites")
	entries, err := os.ReadDir(sitesDir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") && entry.Name() != "assets" {
			return entry.Name()
		}
	}
	return ""
}

// GetVenvPython returns the path to the venv Python interpreter.
// Falls back to "python3" if the venv doesn't exist.
func GetVenvPython(benchPath string) string {
	pythonBin := filepath.Join(benchPath, "env", "bin", "python")
	if _, err := os.Stat(pythonBin); os.IsNotExist(err) {
		return "python3"
	}
	return pythonBin
}

// HasDevbox checks if the project uses devbox.
func HasDevbox(benchPath string) bool {
	devboxJSON := filepath.Join(benchPath, "devbox.json")
	_, err := os.Stat(devboxJSON)
	return err == nil
}

// BuildPythonPath builds the PYTHONPATH for apps in the bench.
// Returns the path string and a slice of individual paths.
func BuildPythonPath(benchPath string) (string, []string) {
	appsDir := filepath.Join(benchPath, "apps")
	pythonPaths := []string{}

	entries, err := os.ReadDir(appsDir)
	if err != nil {
		return "", pythonPaths
	}

	for _, entry := range entries {
		if entry.IsDir() {
			pythonPaths = append(pythonPaths, filepath.Join(appsDir, entry.Name()))
		}
	}

	return strings.Join(pythonPaths, ":"), pythonPaths
}

// MergePythonPath merges the given Python path with the existing PYTHONPATH
// environment variable.
func MergePythonPath(newPath string) string {
	if newPath == "" {
		return os.Getenv("PYTHONPATH")
	}
	existingPath := os.Getenv("PYTHONPATH")
	if existingPath != "" {
		return newPath + ":" + existingPath
	}
	return newPath
}

// ResolveConfigPath returns the appropriate config file path for the project.
func ResolveConfigPath(absPath string) string {
	if config.HasWegToml(absPath) {
		return filepath.Join(absPath, "weg.toml")
	}
	return filepath.Join(absPath, "pyproject.toml")
}

// SitesDir returns the sites directory path for a bench.
func SitesDir(benchPath string) string {
	return filepath.Join(benchPath, "sites")
}

// AppsDir returns the apps directory path for a bench.
func AppsDir(benchPath string) string {
	return filepath.Join(benchPath, "apps")
}

// RequireSite returns an error if no site is available.
func RequireSite(site string) error {
	if site == "" {
		return fmt.Errorf("no site specified and no default site found. Use --site flag")
	}
	return nil
}

// BuildBenchEnv builds common environment variables for bench commands.
func BuildBenchEnv(benchPath, site string) []string {
	env := os.Environ()
	env = append(env, fmt.Sprintf("FRAPPE_BENCH_ROOT=%s", benchPath))
	if site != "" {
		env = append(env, fmt.Sprintf("FRAPPE_SITE=%s", site))
	}
	return env
}

// BuildBenchEnvWithPythonPath builds environment variables including PYTHONPATH.
func BuildBenchEnvWithPythonPath(benchPath, site string) []string {
	env := BuildBenchEnv(benchPath, site)

	pythonPath, _ := BuildPythonPath(benchPath)
	if pythonPath != "" {
		mergedPath := MergePythonPath(pythonPath)
		env = append(env, fmt.Sprintf("PYTHONPATH=%s", mergedPath))
	}

	return env
}
