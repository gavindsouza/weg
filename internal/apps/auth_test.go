package apps

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestGetGitHubToken_EnvPriority(t *testing.T) {
	// Save and reset
	origWeg := os.Getenv("WEG_GITHUB_TOKEN")
	origGH := os.Getenv("GITHUB_TOKEN")
	origGHTok := os.Getenv("GH_TOKEN")
	defer func() {
		os.Setenv("WEG_GITHUB_TOKEN", origWeg)
		os.Setenv("GITHUB_TOKEN", origGH)
		os.Setenv("GH_TOKEN", origGHTok)
		ResetCachedToken()
	}()

	// WEG_GITHUB_TOKEN takes priority
	ResetCachedToken()
	os.Setenv("WEG_GITHUB_TOKEN", "weg-token")
	os.Setenv("GITHUB_TOKEN", "gh-token")
	os.Setenv("GH_TOKEN", "ghtok")
	token := GetGitHubToken()
	if token != "weg-token" {
		t.Errorf("expected weg-token, got %q", token)
	}

	// GITHUB_TOKEN is next
	ResetCachedToken()
	os.Unsetenv("WEG_GITHUB_TOKEN")
	token = GetGitHubToken()
	if token != "gh-token" {
		t.Errorf("expected gh-token, got %q", token)
	}

	// GH_TOKEN is next
	ResetCachedToken()
	os.Unsetenv("GITHUB_TOKEN")
	token = GetGitHubToken()
	if token != "ghtok" {
		t.Errorf("expected ghtok, got %q", token)
	}

	// No token
	ResetCachedToken()
	os.Unsetenv("GH_TOKEN")
	token = GetGitHubToken()
	// May or may not find gh CLI, but shouldn't panic
	_ = token
}

func TestGetPreferredProtocol_Env(t *testing.T) {
	origProto := os.Getenv("WEG_GIT_PROTOCOL")
	defer func() {
		os.Setenv("WEG_GIT_PROTOCOL", origProto)
		ResetCachedProtocol()
	}()

	ResetCachedProtocol()
	os.Setenv("WEG_GIT_PROTOCOL", "ssh")
	if GetPreferredProtocol() != ProtocolSSH {
		t.Error("expected SSH protocol")
	}

	ResetCachedProtocol()
	os.Setenv("WEG_GIT_PROTOCOL", "https")
	if GetPreferredProtocol() != ProtocolHTTPS {
		t.Error("expected HTTPS protocol")
	}

	ResetCachedProtocol()
	os.Unsetenv("WEG_GIT_PROTOCOL")
	// Default should be HTTPS (unless gh CLI says otherwise)
	proto := GetPreferredProtocol()
	_ = proto // just verify no panic
}

func TestExpandShortRef(t *testing.T) {
	origProto := os.Getenv("WEG_GIT_PROTOCOL")
	defer func() {
		os.Setenv("WEG_GIT_PROTOCOL", origProto)
		ResetCachedProtocol()
	}()

	ResetCachedProtocol()
	os.Setenv("WEG_GIT_PROTOCOL", "https")
	url := ExpandShortRef("frappe", "erpnext")
	if url != "https://github.com/frappe/erpnext" {
		t.Errorf("HTTPS: got %q", url)
	}

	ResetCachedProtocol()
	os.Setenv("WEG_GIT_PROTOCOL", "ssh")
	url = ExpandShortRef("frappe", "erpnext")
	if url != "git@github.com:frappe/erpnext.git" {
		t.Errorf("SSH: got %q", url)
	}
}

func TestResolveAppSpec_RespectsSSHProtocol(t *testing.T) {
	origProto := os.Getenv("WEG_GIT_PROTOCOL")
	defer func() {
		os.Setenv("WEG_GIT_PROTOCOL", origProto)
		ResetCachedProtocol()
	}()

	ResetCachedProtocol()
	os.Setenv("WEG_GIT_PROTOCOL", "ssh")
	spec := ResolveAppSpec("frappe/erpnext", "")
	if spec.URL != "git@github.com:frappe/erpnext.git" {
		t.Errorf("expected SSH URL, got %q", spec.URL)
	}
	if spec.Name != "erpnext" {
		t.Errorf("expected name erpnext, got %q", spec.Name)
	}

	// Full URLs are never rewritten
	spec = ResolveAppSpec("https://github.com/frappe/erpnext", "")
	if spec.URL != "https://github.com/frappe/erpnext" {
		t.Errorf("full URL should not be rewritten, got %q", spec.URL)
	}
}

func TestRawURLToAPI(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{
			"https://raw.githubusercontent.com/frappe/erpnext/develop/erpnext/hooks.py",
			"https://api.github.com/repos/frappe/erpnext/contents/erpnext/hooks.py?ref=develop",
		},
		{
			"https://raw.githubusercontent.com/frappe/hrms/version-15/pyproject.toml",
			"https://api.github.com/repos/frappe/hrms/contents/pyproject.toml?ref=version-15",
		},
		{
			"https://example.com/not-github",
			"",
		},
	}
	for _, tt := range tests {
		got := rawURLToAPI(tt.input)
		if got != tt.want {
			t.Errorf("rawURLToAPI(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFetchAuthenticatedContent_WithToken(t *testing.T) {
	origToken := os.Getenv("WEG_GITHUB_TOKEN")
	defer func() {
		os.Setenv("WEG_GITHUB_TOKEN", origToken)
		ResetCachedToken()
	}()

	// Set up a test server that requires auth
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token-123" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Write([]byte("private content"))
	}))
	defer ts.Close()

	// Without token — should fail
	ResetCachedToken()
	os.Unsetenv("WEG_GITHUB_TOKEN")
	os.Unsetenv("GITHUB_TOKEN")
	os.Unsetenv("GH_TOKEN")
	_, err := fetchAuthenticatedContent(ts.URL)
	if err == nil {
		t.Error("expected error without token")
	}

	// With token — should succeed
	ResetCachedToken()
	os.Setenv("WEG_GITHUB_TOKEN", "test-token-123")
	content, err := fetchAuthenticatedContent(ts.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "private content" {
		t.Errorf("got %q, want %q", content, "private content")
	}
}

func TestNewAuthenticatedRequest_SetsHeaders(t *testing.T) {
	origToken := os.Getenv("WEG_GITHUB_TOKEN")
	defer func() {
		os.Setenv("WEG_GITHUB_TOKEN", origToken)
		ResetCachedToken()
	}()

	// With token
	ResetCachedToken()
	os.Setenv("WEG_GITHUB_TOKEN", "my-token")
	req, err := newAuthenticatedRequest("GET", "https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	if req.Header.Get("Authorization") != "Bearer my-token" {
		t.Errorf("Authorization = %q", req.Header.Get("Authorization"))
	}
	if req.Header.Get("User-Agent") != "weg-cli" {
		t.Errorf("User-Agent = %q", req.Header.Get("User-Agent"))
	}

	// Without token — env vars cleared but gh CLI may still provide one
	ResetCachedToken()
	os.Unsetenv("WEG_GITHUB_TOKEN")
	os.Unsetenv("GITHUB_TOKEN")
	os.Unsetenv("GH_TOKEN")
	req, err = newAuthenticatedRequest("GET", "https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	// If gh CLI is installed and authenticated, Authorization will be set — that's fine.
	// We just verify no panic and well-formed header if present.
	if auth := req.Header.Get("Authorization"); auth != "" {
		if len(auth) < 8 || auth[:7] != "Bearer " {
			t.Errorf("malformed Authorization header: %q", auth)
		}
	}
}
