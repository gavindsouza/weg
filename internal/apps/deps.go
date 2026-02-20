package apps

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// AppDependency represents a single app dependency with source info
type AppDependency struct {
	// Name is the app name (e.g., "erpnext")
	Name string

	// Spec is the raw specifier from the dependency source (e.g., "frappe/erpnext")
	Spec string

	// URL is the resolved git URL (may be empty for bare names)
	URL string

	// Branch is the required branch (empty = any/default)
	Branch string
}

// DependencySource indicates where the dependency information was read from
type DependencySource int

const (
	SourceNone      DependencySource = iota
	SourcePyproject                  // pyproject.toml [tool.weg.dependencies.apps]
	SourceHooks                      // hooks.py required_apps
	SourceRemote                     // Fetched from GitHub raw content
)

func (s DependencySource) String() string {
	switch s {
	case SourcePyproject:
		return "pyproject.toml"
	case SourceHooks:
		return "hooks.py"
	case SourceRemote:
		return "remote"
	default:
		return "none"
	}
}

// ReadDependencies reads app dependencies from a local app directory.
// It tries pyproject.toml first, then falls back to hooks.py.
// Returns the list of dependencies and which source was used.
func ReadDependencies(appPath string) ([]AppDependency, DependencySource, error) {
	appName := filepath.Base(appPath)

	// 1. Try pyproject.toml [tool.weg.dependencies.apps]
	deps, err := readDepsFromPyproject(appPath)
	if err == nil && len(deps) > 0 {
		return deps, SourcePyproject, nil
	}

	// 2. Try hooks.py required_apps
	// hooks.py is typically at <appPath>/<appName>/hooks.py
	hooksPath := filepath.Join(appPath, appName, "hooks.py")
	if _, err := os.Stat(hooksPath); os.IsNotExist(err) {
		// Try flat structure
		hooksPath = filepath.Join(appPath, "hooks.py")
	}

	deps, err = readDepsFromHooks(hooksPath)
	if err == nil && len(deps) > 0 {
		return deps, SourceHooks, nil
	}

	return nil, SourceNone, nil
}

// ReadDependenciesRemote reads dependencies from a GitHub repo without cloning.
// It fetches hooks.py and pyproject.toml via raw.githubusercontent.com.
// This is significantly faster than cloning for dependency resolution.
func ReadDependenciesRemote(url, branch string) ([]AppDependency, DependencySource, error) {
	owner, repo, err := parseGitHubURL(url)
	if err != nil {
		return nil, SourceNone, fmt.Errorf("remote dependency reading only supported for GitHub repos: %w", err)
	}

	ref := branch
	if ref == "" {
		ref = "develop" // Frappe ecosystem convention
	}

	appName := repo

	// 1. Try pyproject.toml
	pyprojectURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/pyproject.toml", owner, repo, ref)
	content, err := fetchRawContent(pyprojectURL)
	if err == nil {
		deps, err := parsePyprojectContent(content)
		if err == nil && len(deps) > 0 {
			return deps, SourceRemote, nil
		}
	}

	// 2. Try hooks.py at <repo>/<appName>/hooks.py
	hooksURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s/hooks.py", owner, repo, ref, appName)
	content, err = fetchRawContent(hooksURL)
	if err != nil {
		// Try flat structure
		hooksURL = fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/hooks.py", owner, repo, ref)
		content, err = fetchRawContent(hooksURL)
	}
	if err == nil {
		deps, err := parseHooksContent(content)
		if err == nil && len(deps) > 0 {
			return deps, SourceRemote, nil
		}
	}

	return nil, SourceNone, nil
}

// readDepsFromPyproject reads [tool.weg.dependencies.apps] from pyproject.toml
func readDepsFromPyproject(appPath string) ([]AppDependency, error) {
	pyprojectPath := filepath.Join(appPath, "pyproject.toml")
	content, err := os.ReadFile(pyprojectPath)
	if err != nil {
		return nil, err
	}
	return parsePyprojectContent(string(content))
}

// parsePyprojectContent parses dependencies from pyproject.toml content.
// Looks for [[tool.weg.dependencies.apps]] entries with name, url, branch fields.
func parsePyprojectContent(content string) ([]AppDependency, error) {
	// Simple TOML parsing for [[tool.weg.dependencies.apps]] array
	// Each entry has: name = "...", url = "...", branch = "..."
	var deps []AppDependency

	inAppsSection := false
	currentDep := AppDependency{}

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)

		if line == "[[tool.weg.dependencies.apps]]" {
			if inAppsSection && currentDep.Name != "" {
				deps = append(deps, resolveDep(currentDep))
			}
			inAppsSection = true
			currentDep = AppDependency{}
			continue
		}

		// If we hit another section header, flush current
		if strings.HasPrefix(line, "[") && inAppsSection {
			if currentDep.Name != "" {
				deps = append(deps, resolveDep(currentDep))
			}
			inAppsSection = false
			currentDep = AppDependency{}
			continue
		}

		if !inAppsSection {
			continue
		}

		key, value := parseTomlKV(line)
		switch key {
		case "name":
			currentDep.Name = value
			currentDep.Spec = value
		case "url":
			currentDep.URL = value
		case "branch":
			currentDep.Branch = value
		}
	}

	// Flush last entry
	if inAppsSection && currentDep.Name != "" {
		deps = append(deps, resolveDep(currentDep))
	}

	return deps, nil
}

// parseTomlKV extracts key and unquoted value from a TOML line like: key = "value"
func parseTomlKV(line string) (string, string) {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", ""
	}
	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	value = strings.Trim(value, `"'`)
	return key, value
}

// readDepsFromHooks reads required_apps from hooks.py
func readDepsFromHooks(hooksPath string) ([]AppDependency, error) {
	content, err := os.ReadFile(hooksPath)
	if err != nil {
		return nil, err
	}
	return parseHooksContent(string(content))
}

// requiredAppsPattern matches required_apps = ["..."] in hooks.py
// Handles both single-line and multi-line list definitions
var requiredAppsPattern = regexp.MustCompile(`(?s)required_apps\s*=\s*\[([^\]]*)\]`)

// parseHooksContent extracts required_apps from hooks.py content.
//
// Handles formats like:
//
//	required_apps = ["frappe/erpnext"]
//	required_apps = ["erpnext", "hrms"]
//	required_apps = [
//	    "frappe/erpnext",
//	    "frappe/hrms",
//	]
func parseHooksContent(content string) ([]AppDependency, error) {
	matches := requiredAppsPattern.FindStringSubmatch(content)
	if len(matches) < 2 {
		return nil, nil
	}

	listBody := matches[1]
	var deps []AppDependency

	// Extract quoted strings from the list body
	stringPattern := regexp.MustCompile(`["']([^"']+)["']`)
	for _, m := range stringPattern.FindAllStringSubmatch(listBody, -1) {
		spec := strings.TrimSpace(m[1])
		if spec == "" || spec == "frappe" {
			continue // Skip frappe — it's always implicitly required
		}

		appSpec := ResolveAppSpec(spec, "")
		deps = append(deps, AppDependency{
			Name: appSpec.Name,
			Spec: spec,
			URL:  appSpec.URL,
		})
	}

	return deps, nil
}

// resolveDep fills in URL from Name/Spec if not already set
func resolveDep(dep AppDependency) AppDependency {
	if dep.URL == "" && dep.Name != "" {
		spec := dep.Spec
		if spec == "" {
			spec = dep.Name
		}
		appSpec := ResolveAppSpec(spec, dep.Branch)
		dep.URL = appSpec.URL
		dep.Branch = appSpec.Branch
	}
	return dep
}

// parseGitHubURL extracts owner and repo from a GitHub URL
func parseGitHubURL(url string) (owner, repo string, err error) {
	url = strings.TrimSuffix(url, ".git")
	url = strings.TrimRight(url, "/")

	// https://github.com/owner/repo
	if strings.HasPrefix(url, "https://github.com/") || strings.HasPrefix(url, "http://github.com/") {
		parts := strings.Split(url, "/")
		if len(parts) >= 5 {
			return parts[3], parts[4], nil
		}
	}

	// git@github.com:owner/repo
	if strings.HasPrefix(url, "git@github.com:") {
		path := strings.TrimPrefix(url, "git@github.com:")
		parts := strings.SplitN(path, "/", 2)
		if len(parts) == 2 {
			return parts[0], parts[1], nil
		}
	}

	return "", "", fmt.Errorf("not a GitHub URL: %s", url)
}

// fetchRawContent fetches raw file content from a URL.
// Uses GitHub token authentication when available (required for private repos).
func fetchRawContent(url string) (string, error) {
	return fetchAuthenticatedContent(url)
}

// defaultHTTPClient returns an HTTP client with a reasonable timeout
func defaultHTTPClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

// readLimitedBody reads up to 1MB from an HTTP response
func readLimitedBody(resp *http.Response) (string, error) {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// ReadInstalledApps reads the apps.txt file to get currently installed app names
func ReadInstalledApps(benchPath string) ([]string, error) {
	appsTxtPath := filepath.Join(benchPath, "sites", "apps.txt")
	f, err := os.Open(appsTxtPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var apps []string
	scanner := bufio.NewReader(f)
	for {
		line, err := scanner.ReadString('\n')
		line = strings.TrimSpace(line)
		if line != "" {
			apps = append(apps, line)
		}
		if err != nil {
			break
		}
	}
	return apps, nil
}

// ReadInstalledAppDirs reads which apps are actually cloned in apps/
func ReadInstalledAppDirs(appsDir string) ([]string, error) {
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		return nil, err
	}

	var apps []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			apps = append(apps, entry.Name())
		}
	}
	return apps, nil
}

// FrappeCloudAppInfo represents app info from the Frappe marketplace API
type FrappeCloudAppInfo struct {
	Name      string `json:"name"`
	Repo      string `json:"repo"`
	RepoOwner string `json:"repo_owner"`
	Branch    string `json:"branch"`
}

// LookupAppOnMarketplace attempts to find an app by name on Frappe Cloud marketplace.
// Returns nil if not found or if the API is unavailable.
func LookupAppOnMarketplace(appName string) *FrappeCloudAppInfo {
	url := fmt.Sprintf("https://frappecloud.com/api/method/press.api.marketplace.get_app_info?app=%s", appName)
	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(url)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil
	}
	defer resp.Body.Close()

	var result struct {
		Message FrappeCloudAppInfo `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil
	}

	if result.Message.Repo == "" {
		return nil
	}
	return &result.Message
}
