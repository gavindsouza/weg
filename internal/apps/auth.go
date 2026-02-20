package apps

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// GitProtocol determines how short GitHub references (org/repo) are expanded
type GitProtocol int

const (
	// ProtocolHTTPS expands to https://github.com/org/repo (default)
	ProtocolHTTPS GitProtocol = iota
	// ProtocolSSH expands to git@github.com:org/repo.git
	ProtocolSSH
)

var (
	cachedToken string
	tokenOnce   sync.Once
	cachedProto GitProtocol
	protoOnce   sync.Once
)

// GetGitHubToken returns a GitHub personal access token for API access.
// It checks (in order):
//  1. WEG_GITHUB_TOKEN  — weg-specific override
//  2. GITHUB_TOKEN      — standard GitHub env var (used by CI, Codespaces, etc.)
//  3. GH_TOKEN          — gh CLI env var
//  4. `gh auth token`   — asks the gh CLI for a token (if installed)
//
// Returns empty string if no token is available (public repos only).
func GetGitHubToken() string {
	tokenOnce.Do(func() {
		if t := os.Getenv("WEG_GITHUB_TOKEN"); t != "" {
			cachedToken = t
			return
		}
		if t := os.Getenv("GITHUB_TOKEN"); t != "" {
			cachedToken = t
			return
		}
		if t := os.Getenv("GH_TOKEN"); t != "" {
			cachedToken = t
			return
		}
		// Try gh CLI
		if ghPath, err := exec.LookPath("gh"); err == nil {
			cmd := exec.Command(ghPath, "auth", "token")
			out, err := cmd.Output()
			if err == nil {
				cachedToken = strings.TrimSpace(string(out))
			}
		}
	})
	return cachedToken
}

// ResetCachedToken clears the cached token (for testing)
func ResetCachedToken() {
	tokenOnce = sync.Once{}
	cachedToken = ""
}

// GetPreferredProtocol returns the user's preferred git protocol for short refs.
// Checks (in order):
//  1. WEG_GIT_PROTOCOL env var ("ssh" or "https")
//  2. `gh config get git_protocol` (if gh CLI is installed)
//  3. Defaults to HTTPS
func GetPreferredProtocol() GitProtocol {
	protoOnce.Do(func() {
		if p := os.Getenv("WEG_GIT_PROTOCOL"); p != "" {
			if strings.EqualFold(p, "ssh") {
				cachedProto = ProtocolSSH
				return
			}
			cachedProto = ProtocolHTTPS
			return
		}
		// Try gh CLI config
		if ghPath, err := exec.LookPath("gh"); err == nil {
			cmd := exec.Command(ghPath, "config", "get", "git_protocol")
			out, err := cmd.Output()
			if err == nil && strings.TrimSpace(string(out)) == "ssh" {
				cachedProto = ProtocolSSH
				return
			}
		}
		cachedProto = ProtocolHTTPS
	})
	return cachedProto
}

// ResetCachedProtocol clears the cached protocol (for testing)
func ResetCachedProtocol() {
	protoOnce = sync.Once{}
	cachedProto = ProtocolHTTPS
}

// ExpandShortRef expands an "org/repo" shorthand into a full git URL,
// respecting the user's preferred protocol.
func ExpandShortRef(org, repo string) string {
	if GetPreferredProtocol() == ProtocolSSH {
		return fmt.Sprintf("git@github.com:%s/%s.git", org, repo)
	}
	return fmt.Sprintf("https://github.com/%s/%s", org, repo)
}

// newAuthenticatedRequest creates an HTTP request with GitHub token auth if available.
func newAuthenticatedRequest(method, url string) (*http.Request, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	token := GetGitHubToken()
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	req.Header.Set("User-Agent", "weg-cli")
	return req, nil
}

// fetchAuthenticatedContent fetches content from a URL, using GitHub token auth when available.
// Falls back to unauthenticated requests if no token is set.
// For GitHub URLs, uses the GitHub API contents endpoint which works for private repos.
func fetchAuthenticatedContent(url string) (string, error) {
	req, err := newAuthenticatedRequest("GET", url)
	if err != nil {
		return "", err
	}

	// If this is a raw.githubusercontent.com URL and we have a token,
	// rewrite to use the GitHub API (which supports private repos)
	if token := GetGitHubToken(); token != "" && strings.Contains(url, "raw.githubusercontent.com") {
		apiURL := rawURLToAPI(url)
		if apiURL != "" {
			req, err = newAuthenticatedRequest("GET", apiURL)
			if err != nil {
				return "", err
			}
			// Ask for raw content (not JSON-wrapped)
			req.Header.Set("Accept", "application/vnd.github.v3.raw")
		}
	}

	client := defaultHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("HTTP 404 for %s", url)
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		return "", fmt.Errorf("HTTP %d for %s (authentication required — set GITHUB_TOKEN or run `gh auth login`)", resp.StatusCode, url)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}

	return readLimitedBody(resp)
}

// rawURLToAPI converts a raw.githubusercontent.com URL to a GitHub API contents URL.
//
// Input:  https://raw.githubusercontent.com/owner/repo/ref/path/to/file
// Output: https://api.github.com/repos/owner/repo/contents/path/to/file?ref=ref
func rawURLToAPI(rawURL string) string {
	const prefix = "https://raw.githubusercontent.com/"
	if !strings.HasPrefix(rawURL, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(rawURL, prefix)
	// rest = "owner/repo/ref/path/to/file"
	parts := strings.SplitN(rest, "/", 4)
	if len(parts) < 4 {
		return ""
	}
	owner, repo, ref, path := parts[0], parts[1], parts[2], parts[3]
	return fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s", owner, repo, path, ref)
}
