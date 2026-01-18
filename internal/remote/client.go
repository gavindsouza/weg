/*
Copyright © 2025 Gavin <me@gavv.in>

Frappe REST API client for remote site operations.
*/
package remote

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

// Client is a Frappe REST API client
type Client struct {
	BaseURL    string
	APIKey     string
	APISecret  string
	HTTPClient *http.Client
}

// NewClient creates a new Frappe API client
func NewClient(baseURL, apiKey, apiSecret string) *Client {
	// Ensure URL doesn't have trailing slash
	baseURL = strings.TrimRight(baseURL, "/")

	return &Client{
		BaseURL:   baseURL,
		APIKey:    apiKey,
		APISecret: apiSecret,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewClientFromConfig creates a client from site config and credentials
func NewClientFromConfig(config *SiteConfig, creds *Credentials) *Client {
	return NewClient(
		config.Site.URL,
		creds.Auth.APIKey,
		creds.Auth.APISecret,
	)
}

// APIResponse represents a standard Frappe API response
type APIResponse struct {
	Message interface{} `json:"message"`
	Exc     string      `json:"exc,omitempty"`
	ExcType string      `json:"exc_type,omitempty"`
}

// DocListResponse represents a response from get_list
type DocListResponse struct {
	Data []map[string]interface{} `json:"data"`
}

// request makes an authenticated request to the Frappe API
func (c *Client) request(method, endpoint string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	reqURL := c.BaseURL + endpoint
	req, err := http.NewRequest(method, reqURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// API key authentication
	if c.APIKey != "" && c.APISecret != "" {
		req.Header.Set("Authorization", fmt.Sprintf("token %s:%s", c.APIKey, c.APISecret))
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var apiResp APIResponse
		if json.Unmarshal(respBody, &apiResp) == nil && apiResp.Exc != "" {
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, apiResp.Exc)
		}
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// Ping tests the connection to the Frappe site
func (c *Client) Ping() error {
	_, err := c.request("GET", "/api/method/frappe.ping", nil)
	return err
}

// GetDoc retrieves a single document
func (c *Client) GetDoc(doctype, name string) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("/api/resource/%s/%s", url.PathEscape(doctype), url.PathEscape(name))

	respBody, err := c.request("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Data, nil
}

// GetList retrieves a list of documents
func (c *Client) GetList(doctype string, filters map[string]interface{}, fields []string, limit int) ([]map[string]interface{}, error) {
	endpoint := fmt.Sprintf("/api/resource/%s", url.PathEscape(doctype))

	// Build query parameters
	params := url.Values{}
	if len(filters) > 0 {
		filtersJSON, _ := json.Marshal(filters)
		params.Set("filters", string(filtersJSON))
	}
	if len(fields) > 0 {
		fieldsJSON, _ := json.Marshal(fields)
		params.Set("fields", string(fieldsJSON))
	}
	if limit > 0 {
		params.Set("limit_page_length", fmt.Sprintf("%d", limit))
	}

	if len(params) > 0 {
		endpoint += "?" + params.Encode()
	}

	respBody, err := c.request("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Data, nil
}

// GetAll retrieves all documents of a type (handles pagination)
func (c *Client) GetAll(doctype string, filters map[string]interface{}, fields []string) ([]map[string]interface{}, error) {
	var allDocs []map[string]interface{}
	pageSize := 100
	offset := 0

	for {
		endpoint := fmt.Sprintf("/api/resource/%s", url.PathEscape(doctype))

		params := url.Values{}
		if len(filters) > 0 {
			filtersJSON, _ := json.Marshal(filters)
			params.Set("filters", string(filtersJSON))
		}
		if len(fields) > 0 {
			fieldsJSON, _ := json.Marshal(fields)
			params.Set("fields", string(fieldsJSON))
		}
		params.Set("limit_page_length", fmt.Sprintf("%d", pageSize))
		params.Set("limit_start", fmt.Sprintf("%d", offset))

		endpoint += "?" + params.Encode()

		respBody, err := c.request("GET", endpoint, nil)
		if err != nil {
			return nil, err
		}

		var result struct {
			Data []map[string]interface{} `json:"data"`
		}
		if err := json.Unmarshal(respBody, &result); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}

		allDocs = append(allDocs, result.Data...)

		if len(result.Data) < pageSize {
			break
		}
		offset += pageSize
	}

	return allDocs, nil
}

// CallMethod calls a whitelisted Frappe method
func (c *Client) CallMethod(method string, args map[string]interface{}) (interface{}, error) {
	endpoint := fmt.Sprintf("/api/method/%s", method)

	// Frappe requires a body for POST requests; use empty object if no args
	body := args
	if body == nil {
		body = map[string]interface{}{}
	}

	respBody, err := c.request("POST", endpoint, body)
	if err != nil {
		return nil, err
	}

	var result APIResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Message, nil
}

// InsertDoc creates a new document
func (c *Client) InsertDoc(doctype string, doc map[string]interface{}) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("/api/resource/%s", url.PathEscape(doctype))

	respBody, err := c.request("POST", endpoint, doc)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Data, nil
}

// UpdateDoc updates an existing document
func (c *Client) UpdateDoc(doctype, name string, doc map[string]interface{}) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("/api/resource/%s/%s", url.PathEscape(doctype), url.PathEscape(name))

	respBody, err := c.request("PUT", endpoint, doc)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Data, nil
}

// DeleteDoc deletes a document
func (c *Client) DeleteDoc(doctype, name string) error {
	endpoint := fmt.Sprintf("/api/resource/%s/%s", url.PathEscape(doctype), url.PathEscape(name))
	_, err := c.request("DELETE", endpoint, nil)
	return err
}

// GetVersions retrieves Version records for a document
func (c *Client) GetVersions(doctype, name string) ([]map[string]interface{}, error) {
	filters := map[string]interface{}{
		"ref_doctype": doctype,
		"docname":     name,
	}
	fields := []string{"name", "owner", "creation", "data"}

	return c.GetAll("Version", filters, fields)
}

// GetInstalledApps retrieves the list of installed apps
func (c *Client) GetInstalledApps() ([]map[string]interface{}, error) {
	result, err := c.CallMethod("frappe.client.get_list", map[string]interface{}{
		"doctype": "Installed Application",
		"fields":  []string{"app_name", "app_version", "git_branch"},
	})
	if err != nil {
		// Fallback: try Module Def to infer apps
		return c.GetAll("Module Def", nil, []string{"name", "app_name", "module_name"})
	}

	if apps, ok := result.([]interface{}); ok {
		var appList []map[string]interface{}
		for _, app := range apps {
			if appMap, ok := app.(map[string]interface{}); ok {
				appList = append(appList, appMap)
			}
		}
		return appList, nil
	}

	return nil, fmt.Errorf("unexpected response format")
}

// GetModules retrieves all Module Def records
func (c *Client) GetModules() ([]map[string]interface{}, error) {
	return c.GetAll("Module Def", nil, []string{"name", "app_name", "module_name"})
}

// GetFrappeVersion retrieves the Frappe version
func (c *Client) GetFrappeVersion() (string, error) {
	result, err := c.CallMethod("frappe.utils.change_log.get_versions", nil)
	if err != nil {
		return "", err
	}

	if versions, ok := result.(map[string]interface{}); ok {
		if frappe, ok := versions["frappe"].(map[string]interface{}); ok {
			if version, ok := frappe["version"].(string); ok {
				return version, nil
			}
		}
	}

	return "", fmt.Errorf("could not determine Frappe version")
}

// VersionRecord represents a single version history entry
type VersionRecord struct {
	Name       string `json:"name"`
	RefDoctype string `json:"ref_doctype"`
	Docname    string `json:"docname"`
	Owner      string `json:"owner"`
	Creation   string `json:"creation"`
	Data       string `json:"data"` // JSON string of version data
}

// GetAllVersions retrieves all Version records for given doctypes
func (c *Client) GetAllVersions(doctypes []string) ([]VersionRecord, error) {
	// Build filter for multiple doctypes
	filters := map[string]interface{}{
		"ref_doctype": []interface{}{"in", doctypes},
	}

	docs, err := c.GetAll("Version", filters, []string{"name", "ref_doctype", "docname", "owner", "creation", "data"})
	if err != nil {
		return nil, err
	}

	var versions []VersionRecord
	for _, doc := range docs {
		v := VersionRecord{
			Name:       getString(doc, "name"),
			RefDoctype: getString(doc, "ref_doctype"),
			Docname:    getString(doc, "docname"),
			Owner:      getString(doc, "owner"),
			Creation:   getString(doc, "creation"),
			Data:       getString(doc, "data"),
		}
		if v.Name != "" && v.RefDoctype != "" && v.Docname != "" {
			versions = append(versions, v)
		}
	}

	return versions, nil
}

// UserInfo contains basic user information
type UserInfo struct {
	Email    string
	FullName string
}

// GetUsers retrieves user information for a list of emails/usernames
func (c *Client) GetUsers(emails []string) (map[string]UserInfo, error) {
	if len(emails) == 0 {
		return make(map[string]UserInfo), nil
	}

	// Deduplicate emails
	seen := make(map[string]bool)
	var uniqueEmails []string
	for _, email := range emails {
		if !seen[email] && email != "" {
			seen[email] = true
			uniqueEmails = append(uniqueEmails, email)
		}
	}

	// Fetch users
	filters := map[string]interface{}{
		"name": []interface{}{"in", uniqueEmails},
	}
	docs, err := c.GetAll("User", filters, []string{"name", "full_name", "email"})
	if err != nil {
		return nil, err
	}

	users := make(map[string]UserInfo)
	for _, doc := range docs {
		name := getString(doc, "name")
		fullName := getString(doc, "full_name")
		email := getString(doc, "email")

		if name != "" {
			// Use full_name if available, otherwise derive from email
			if fullName == "" {
				fullName = deriveNameFromEmail(name)
			}
			users[name] = UserInfo{
				Email:    email,
				FullName: fullName,
			}
		}
	}

	// Add fallback for users not found
	for _, email := range uniqueEmails {
		if _, exists := users[email]; !exists {
			users[email] = UserInfo{
				Email:    email,
				FullName: deriveNameFromEmail(email),
			}
		}
	}

	return users, nil
}

// GetDocTypeModules retrieves the module for each specified DocType
// Returns a map of doctype name -> module name
func (c *Client) GetDocTypeModules(doctypes []string) (map[string]string, error) {
	if len(doctypes) == 0 {
		return make(map[string]string), nil
	}

	// Deduplicate
	seen := make(map[string]bool)
	var uniqueDocTypes []string
	for _, dt := range doctypes {
		if !seen[dt] && dt != "" {
			seen[dt] = true
			uniqueDocTypes = append(uniqueDocTypes, dt)
		}
	}

	// Fetch DocTypes with their modules
	filters := map[string]interface{}{
		"name": []interface{}{"in", uniqueDocTypes},
	}
	docs, err := c.GetAll("DocType", filters, []string{"name", "module"})
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, doc := range docs {
		name := getString(doc, "name")
		module := getString(doc, "module")
		if name != "" {
			if module == "" {
				module = "_"
			}
			result[name] = module
		}
	}

	return result, nil
}

// deriveNameFromEmail extracts a readable name from an email address
func deriveNameFromEmail(email string) string {
	// Handle special cases
	if email == "Administrator" {
		return "Administrator"
	}
	if email == "Guest" {
		return "Guest"
	}

	// Extract part before @
	parts := strings.Split(email, "@")
	if len(parts) == 0 || parts[0] == "" {
		return email
	}

	name := parts[0]
	// Replace dots and underscores with spaces and title case
	name = strings.ReplaceAll(name, ".", " ")
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "-", " ")

	// Title case each word
	words := strings.Fields(name)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
		}
	}

	return strings.Join(words, " ")
}
