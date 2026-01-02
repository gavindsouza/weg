package cloud

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/gavindsouza/weg/internal/fsutil"
)

const (
	FrappeCloudAPI = "https://frappecloud.com/api"
	TokenFile      = ".weg/cloud-token.json"
)

// Token represents the authentication token
type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	TeamID       string    `json:"team_id,omitempty"`
}

// Client is the Frappe Cloud API client
type Client struct {
	BaseURL    string
	Token      *Token
	HTTPClient *http.Client
}

// NewClient creates a new Frappe Cloud client
func NewClient(apiKey string) *Client {
	c := &Client{
		BaseURL: FrappeCloudAPI,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	if apiKey != "" {
		c.Token = &Token{
			AccessToken: apiKey,
			ExpiresAt:   time.Now().Add(365 * 24 * time.Hour),
		}
	}
	return c
}

// LoadToken loads the stored token
func (c *Client) LoadToken() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	tokenPath := filepath.Join(home, TokenFile)
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		return fmt.Errorf("not logged in: run 'weg cloud login' first")
	}

	var token Token
	if err := json.Unmarshal(data, &token); err != nil {
		return fmt.Errorf("invalid token file: %w", err)
	}

	if time.Now().After(token.ExpiresAt) {
		return fmt.Errorf("token expired: run 'weg cloud login' to refresh")
	}

	c.Token = &token
	return nil
}

// SaveToken saves the token to disk
func (c *Client) SaveToken(token *Token) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	tokenPath := filepath.Join(home, TokenFile)
	if err := os.MkdirAll(filepath.Dir(tokenPath), 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}

	return fsutil.AtomicWrite(tokenPath, data, 0600)
}

// ClearToken removes the stored token
func (c *Client) ClearToken() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	tokenPath := filepath.Join(home, TokenFile)
	return os.Remove(tokenPath)
}

// IsLoggedIn checks if a valid token exists
func (c *Client) IsLoggedIn() bool {
	if err := c.LoadToken(); err != nil {
		return false
	}
	return c.Token != nil && time.Now().Before(c.Token.ExpiresAt)
}

// LoginWithAPIKey authenticates with an API key
func (c *Client) LoginWithAPIKey(apiKey, apiSecret string) error {
	// For API key auth, we store it directly as the token
	token := &Token{
		AccessToken: fmt.Sprintf("%s:%s", apiKey, apiSecret),
		ExpiresAt:   time.Now().Add(365 * 24 * time.Hour), // API keys don't expire
	}

	c.Token = token
	return c.SaveToken(token)
}

// GetDeviceCode initiates OAuth device flow (for future implementation)
func (c *Client) GetDeviceCode() (*DeviceCodeResponse, error) {
	// This would initiate OAuth device flow
	// For now, we use API key authentication
	return nil, fmt.Errorf("OAuth device flow not yet implemented. Use API key authentication.")
}

// DeviceCodeResponse represents the device code response
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURL string `json:"verification_url"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// doRequest performs an authenticated API request
func (c *Client) doRequest(method, path string, body url.Values) (*http.Response, error) {
	if c.Token == nil {
		return nil, fmt.Errorf("not authenticated")
	}

	reqURL := c.BaseURL + path

	var req *http.Request
	var err error

	if body != nil {
		req, err = http.NewRequest(method, reqURL, nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = body.Encode()
	} else {
		req, err = http.NewRequest(method, reqURL, nil)
		if err != nil {
			return nil, err
		}
	}

	req.Header.Set("Authorization", "token "+c.Token.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	return c.HTTPClient.Do(req)
}

// User represents the current user info
type User struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

// GetCurrentUser returns the current authenticated user
func (c *Client) GetCurrentUser() (*User, error) {
	resp, err := c.doRequest("GET", "/method/frappe.auth.get_logged_user", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("authentication failed")
	}

	var result struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &User{Email: result.Message}, nil
}

// SaveCredentials saves the API key to disk
func SaveCredentials(homeDir, apiKey string) error {
	configDir := filepath.Join(homeDir, ".weg")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return err
	}

	token := Token{
		AccessToken: apiKey,
		ExpiresAt:   time.Now().Add(365 * 24 * time.Hour),
	}

	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}

	return fsutil.AtomicWrite(filepath.Join(configDir, "cloud-token.json"), data, 0600)
}

// LoadCredentials loads the API key from disk
func LoadCredentials(homeDir string) (string, error) {
	tokenPath := filepath.Join(homeDir, ".weg", "cloud-token.json")
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		return "", err
	}

	var token Token
	if err := json.Unmarshal(data, &token); err != nil {
		return "", err
	}

	if time.Now().After(token.ExpiresAt) {
		return "", fmt.Errorf("token expired")
	}

	return token.AccessToken, nil
}

// RemoveCredentials removes stored credentials
func RemoveCredentials(homeDir string) error {
	tokenPath := filepath.Join(homeDir, ".weg", "cloud-token.json")
	return os.Remove(tokenPath)
}
