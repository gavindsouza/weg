package apps

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// AppSpec represents a resolved app specification with URL, name, branch, and locality info.
// This is the canonical way to parse app identifiers throughout weg.
type AppSpec struct {
	// Name is the derived app name (e.g., "erpnext")
	Name string

	// URL is the resolved git URL or absolute local path.
	// Empty if only a bare name was given with no URL resolution possible.
	URL string

	// Branch is the git branch or tag to use. Empty means default branch.
	Branch string

	// IsLocal is true when the spec points to a local filesystem path
	IsLocal bool
}

// String returns a human-readable representation of the AppSpec
func (s AppSpec) String() string {
	if s.IsLocal {
		return fmt.Sprintf("%s (local: %s)", s.Name, s.URL)
	}
	if s.Branch != "" {
		return fmt.Sprintf("%s (%s@%s)", s.Name, s.URL, s.Branch)
	}
	if s.URL != "" {
		return fmt.Sprintf("%s (%s)", s.Name, s.URL)
	}
	return s.Name
}

// Source returns the URL or path to use for cloning/referencing
func (s AppSpec) Source() string {
	return s.URL
}

// shortGitHubPattern matches "org/repo" style references
var shortGitHubPattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+$`)

// ResolveAppSpec parses an app specifier string and an optional branch into a canonical AppSpec.
//
// The specifier can be:
//   - A full git URL: https://github.com/frappe/erpnext, git@github.com:frappe/erpnext.git
//   - A short GitHub ref: frappe/erpnext  (expands to https://github.com/frappe/erpnext)
//   - A bare app name: erpnext (no URL resolution — name only)
//   - A local filesystem path: ./myapp, ../myapp, /home/user/myapp
//
// If branch is empty, it is left unset (meaning default branch).
// Both branch names and tags are accepted — git treats them the same for clone/checkout.
func ResolveAppSpec(spec string, branch string) AppSpec {
	// 1. Local filesystem path
	if isLocalPath(spec) {
		absPath, err := filepath.Abs(spec)
		if err != nil {
			absPath = spec
		}
		return AppSpec{
			Name:    filepath.Base(absPath),
			URL:     absPath,
			Branch:  branch,
			IsLocal: true,
		}
	}

	// 2. Full git URL (https://, http://, git@, ssh://)
	if isGitURL(spec) {
		return AppSpec{
			Name:   extractNameFromURL(spec),
			URL:    spec,
			Branch: branch,
		}
	}

	// 3. Short GitHub reference (org/repo)
	if shortGitHubPattern.MatchString(spec) {
		parts := strings.SplitN(spec, "/", 2)
		url := ExpandShortRef(parts[0], parts[1])
		return AppSpec{
			Name:   parts[1],
			URL:    url,
			Branch: branch,
		}
	}

	// 4. Bare app name — no URL, just the name
	return AppSpec{
		Name:   spec,
		URL:    "",
		Branch: branch,
	}
}

// isLocalPath checks if a string looks like a local filesystem path
func isLocalPath(spec string) bool {
	// Explicit relative or absolute paths
	if strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../") || strings.HasPrefix(spec, "/") {
		return true
	}
	// Home directory expansion
	if strings.HasPrefix(spec, "~") {
		return true
	}
	// Check if path actually exists on disk (handles bare directory names like "myapp"
	// that happen to exist locally — but only if they're NOT already matched as a
	// git URL or org/repo)
	info, err := os.Stat(spec)
	if err == nil && info.IsDir() {
		return true
	}
	return false
}

// isGitURL checks if a string is a git remote URL
func isGitURL(spec string) bool {
	return strings.HasPrefix(spec, "http://") ||
		strings.HasPrefix(spec, "https://") ||
		strings.HasPrefix(spec, "git@") ||
		strings.HasPrefix(spec, "ssh://") ||
		strings.HasPrefix(spec, "git://")
}

// extractNameFromURL extracts the app/repo name from a git URL
//
// Examples:
//
//	https://github.com/frappe/erpnext       -> erpnext
//	https://github.com/frappe/erpnext.git   -> erpnext
//	git@github.com:frappe/erpnext.git       -> erpnext
//	ssh://git@github.com/frappe/erpnext     -> erpnext
func extractNameFromURL(url string) string {
	url = strings.TrimSuffix(url, ".git")
	url = strings.TrimRight(url, "/")

	// Handle git@host:org/repo format
	if strings.HasPrefix(url, "git@") {
		if colonIdx := strings.LastIndex(url, ":"); colonIdx != -1 {
			url = url[colonIdx+1:]
		}
	}

	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return url
}
