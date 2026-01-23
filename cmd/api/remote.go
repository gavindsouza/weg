package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// RemoteClient handles HTTP API calls to remote Frappe sites
type RemoteClient struct {
	BaseURL   string
	APIKey    string
	APISecret string
	client    *http.Client
}

// NewRemoteClient creates a new remote API client
func NewRemoteClient(baseURL, apiKey, apiSecret string) *RemoteClient {
	// Ensure URL doesn't have trailing slash
	baseURL = strings.TrimRight(baseURL, "/")

	return &RemoteClient{
		BaseURL:   baseURL,
		APIKey:    apiKey,
		APISecret: apiSecret,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// RemoteResult represents the API response
type RemoteResult struct {
	Success bool
	Data    any
	Error   string
}

// GetDoc fetches a single document
func (c *RemoteClient) GetDoc(doctype, name string) (*RemoteResult, error) {
	endpoint := fmt.Sprintf("/api/resource/%s/%s", url.PathEscape(doctype), url.PathEscape(name))
	return c.doRequest("GET", endpoint, nil)
}

// GetList fetches a list of documents
func (c *RemoteClient) GetList(doctype string, filters map[string]any, fields []string, limit int, orderBy string) (*RemoteResult, error) {
	endpoint := fmt.Sprintf("/api/resource/%s", url.PathEscape(doctype))

	// Build query parameters
	params := url.Values{}
	if filters != nil {
		filtersJSON, _ := json.Marshal(filters)
		params.Set("filters", string(filtersJSON))
	}
	if fields != nil {
		fieldsJSON, _ := json.Marshal(fields)
		params.Set("fields", string(fieldsJSON))
	}
	if limit > 0 {
		params.Set("limit_page_length", fmt.Sprintf("%d", limit))
	}
	if orderBy != "" {
		params.Set("order_by", orderBy)
	}

	if len(params) > 0 {
		endpoint += "?" + params.Encode()
	}

	return c.doRequest("GET", endpoint, nil)
}

// Create creates a new document
func (c *RemoteClient) Create(doctype string, data map[string]any) (*RemoteResult, error) {
	endpoint := fmt.Sprintf("/api/resource/%s", url.PathEscape(doctype))
	return c.doRequest("POST", endpoint, data)
}

// Update updates an existing document
func (c *RemoteClient) Update(doctype, name string, data map[string]any) (*RemoteResult, error) {
	endpoint := fmt.Sprintf("/api/resource/%s/%s", url.PathEscape(doctype), url.PathEscape(name))
	return c.doRequest("PUT", endpoint, data)
}

// Delete deletes a document
func (c *RemoteClient) Delete(doctype, name string) (*RemoteResult, error) {
	endpoint := fmt.Sprintf("/api/resource/%s/%s", url.PathEscape(doctype), url.PathEscape(name))
	return c.doRequest("DELETE", endpoint, nil)
}

// Call invokes a whitelisted method
func (c *RemoteClient) Call(method string, args map[string]any) (*RemoteResult, error) {
	endpoint := fmt.Sprintf("/api/method/%s", method)
	return c.doRequest("POST", endpoint, args)
}

// doRequest performs the HTTP request with authentication
func (c *RemoteClient) doRequest(method, endpoint string, body any) (*RemoteResult, error) {
	fullURL := c.BaseURL + endpoint

	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest(method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", fmt.Sprintf("token %s:%s", c.APIKey, c.APISecret))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse response
	var result struct {
		Data    any    `json:"data"`
		Message any    `json:"message"`
		Exc     string `json:"exc"`
		ExcType string `json:"exc_type"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		// If not JSON, return raw response as error
		if resp.StatusCode >= 400 {
			return &RemoteResult{Success: false, Error: string(respBody)}, nil
		}
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for errors
	if resp.StatusCode >= 400 || result.Exc != "" {
		errMsg := result.Exc
		if errMsg == "" {
			errMsg = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		return &RemoteResult{Success: false, Error: errMsg}, nil
	}

	// Return data or message
	data := result.Data
	if data == nil {
		data = result.Message
	}

	return &RemoteResult{Success: true, Data: data}, nil
}
