package cloud

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	DefaultCloudAPI = "https://cloud.frappe.io/api"
)

// Token represents the authentication token
type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	Team         string    `json:"team,omitempty"`
	CloudURL     string    `json:"cloud_url,omitempty"`
}

// Client is the Frappe Cloud API client
type Client struct {
	BaseURL    string
	Token      *Token
	Team       string
	HTTPClient *http.Client
}

// NewClient creates a new Frappe Cloud client
func NewClient(apiKey string) *Client {
	return NewClientWithURL(apiKey, DefaultCloudAPI)
}

// NewClientWithURL creates a new Frappe Cloud client with a custom URL
func NewClientWithURL(apiKey, cloudURL string) *Client {
	if cloudURL == "" {
		cloudURL = DefaultCloudAPI
	}
	c := &Client{
		BaseURL: cloudURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	if apiKey != "" {
		c.Token = &Token{
			AccessToken: apiKey,
			ExpiresAt:   time.Now().Add(365 * 24 * time.Hour),
			CloudURL:    cloudURL,
		}
	}
	return c
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

	req.Header.Set("Authorization", "Token "+c.Token.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	return c.HTTPClient.Do(req)
}

// User represents the current user info
type User struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	Team  string `json:"team"`
}

// GetCurrentUser returns the current authenticated user
func (c *Client) GetCurrentUser() (*User, error) {
	resp, err := c.doRequest("GET", "/method/press.api.account.me", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("authentication failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Message struct {
			User string `json:"user"`
			Team string `json:"team"`
		} `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %s", string(body))
	}

	return &User{Email: result.Message.User, Team: result.Message.Team}, nil
}

// TeamInfo represents a team the user belongs to
type TeamInfo struct {
	Name    string `json:"name"`
	Title   string `json:"title"`
	Enabled bool   `json:"enabled"`
}

// GetTeams returns the teams the user belongs to
func (c *Client) GetTeams() ([]TeamInfo, error) {
	resp, err := c.doRequest("GET", "/method/press.api.account.get_teams", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get teams (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Message []TeamInfo `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse teams response: %s", string(body))
	}

	return result.Message, nil
}

// SetTeam sets the active team for API requests
func (c *Client) SetTeam(team string) {
	c.Team = team
	if c.Token != nil {
		c.Token.Team = team
	}
}
