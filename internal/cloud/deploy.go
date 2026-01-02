package cloud

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Site represents a Frappe Cloud site
type Site struct {
	Name      string   `json:"name"`
	Status    string   `json:"status"`
	Plan      string   `json:"plan"`
	Region    string   `json:"region"`
	Apps      []string `json:"apps"`
	CreatedAt string   `json:"creation"`
}

// Bench represents a Frappe Cloud bench/release group
type Bench struct {
	Name    string   `json:"name"`
	Title   string   `json:"title"`
	Version string   `json:"version"`
	Apps    []string `json:"apps"`
	Status  string   `json:"status"`
}

// DeployOptions configures a deployment
type DeployOptions struct {
	SiteName string
	Apps     []string
	Branch   string
	Message  string
}

// Deployment represents a deployment job
type Deployment struct {
	ID         string `json:"name"`
	Site       string `json:"site"`
	Status     string `json:"status"`
	StartedAt  string `json:"creation"`
	FinishedAt string `json:"finished_at"`
	Duration   string `json:"duration"`
	Error      string `json:"error"`
}

// ListSites returns all sites for the authenticated user (optionally filtered by team)
func (c *Client) ListSites(team string) ([]Site, error) {
	resp, err := c.doRequest("GET", "/method/press.api.site.all", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s", string(body))
	}

	var result struct {
		Message []Site `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Message, nil
}

// GetSite returns details for a specific site
func (c *Client) GetSite(siteName string) (*Site, error) {
	params := url.Values{}
	params.Set("name", siteName)

	resp, err := c.doRequest("GET", "/method/press.api.site.get", params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s", string(body))
	}

	var result struct {
		Message Site `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result.Message, nil
}

// BenchInfo represents a bench with additional info for listing
type BenchInfo struct {
	Name          string `json:"name"`
	Status        string `json:"status"`
	FrappeVersion string `json:"frappe_version"`
	AppCount      int    `json:"app_count"`
	SiteCount     int    `json:"site_count"`
}

// ListBenches returns all benches/release groups (optionally filtered by team)
func (c *Client) ListBenches(team string) ([]BenchInfo, error) {
	resp, err := c.doRequest("GET", "/method/press.api.bench.all", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s", string(body))
	}

	var result struct {
		Message []BenchInfo `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Message, nil
}

// DeployToSite triggers a deployment to a site and returns the deployment info
func (c *Client) DeployToSite(siteName, appName string) (*Deployment, error) {
	params := url.Values{}
	params.Set("name", siteName)

	resp, err := c.doRequest("POST", "/method/press.api.site.deploy", params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("deployment failed: %s", string(body))
	}

	var result struct {
		Message Deployment `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result.Message, nil
}

// GetDeployment returns a deployment by ID
func (c *Client) GetDeployment(deployID string) (*Deployment, error) {
	params := url.Values{}
	params.Set("name", deployID)

	resp, err := c.doRequest("GET", "/method/press.api.site.deploy_status", params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s", string(body))
	}

	var result struct {
		Message Deployment `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result.Message, nil
}

// ListDeployments returns recent deployments (optionally filtered by site)
func (c *Client) ListDeployments(siteName string) ([]Deployment, error) {
	params := url.Values{}
	if siteName != "" {
		params.Set("site", siteName)
	}

	resp, err := c.doRequest("GET", "/method/press.api.site.deployments", params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s", string(body))
	}

	var result struct {
		Message []Deployment `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Message, nil
}

// GetSiteLogs returns recent logs for a site
func (c *Client) GetSiteLogs(siteName, logType string, lines int) ([]string, error) {
	logs, err := c.SiteLogs(siteName, logType)
	if err != nil {
		return nil, err
	}
	return strings.Split(logs, "\n"), nil
}

// StreamSiteLogs streams logs from a site (returns channels for log lines and errors)
func (c *Client) StreamSiteLogs(siteName, logType string) (<-chan string, <-chan error) {
	logChan := make(chan string)
	errChan := make(chan error, 1)

	// Note: Real implementation would use websockets or SSE
	// This is a placeholder that closes immediately
	go func() {
		close(logChan)
		close(errChan)
	}()

	return logChan, errChan
}

// Deploy triggers a deployment for a site
func (c *Client) Deploy(opts DeployOptions) error {
	params := url.Values{}
	params.Set("name", opts.SiteName)
	if opts.Message != "" {
		params.Set("message", opts.Message)
	}

	resp, err := c.doRequest("POST", "/method/press.api.site.deploy", params)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("deployment failed: %s", string(body))
	}

	return nil
}

// GetDeployStatus returns the status of a deployment
func (c *Client) GetDeployStatus(siteName string) (string, error) {
	site, err := c.GetSite(siteName)
	if err != nil {
		return "", err
	}
	return site.Status, nil
}

// SiteLogs returns recent logs for a site
func (c *Client) SiteLogs(siteName string, logType string) (string, error) {
	params := url.Values{}
	params.Set("name", siteName)
	params.Set("log_type", logType) // web, worker, scheduler

	resp, err := c.doRequest("GET", "/method/press.api.site.logs", params)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error: %s", string(body))
	}

	var result struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Message, nil
}

// UpdateApp updates an app on a site to a specific version/commit
func (c *Client) UpdateApp(siteName, appName, version string) error {
	params := url.Values{}
	params.Set("name", siteName)
	params.Set("app", appName)
	if version != "" {
		params.Set("hash", version)
	}

	resp, err := c.doRequest("POST", "/method/press.api.site.update_app", params)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("update failed: %s", string(body))
	}

	return nil
}

// FormatSiteList formats sites for display
func FormatSiteList(sites []Site) string {
	if len(sites) == 0 {
		return "No sites found."
	}

	var sb strings.Builder
	sb.WriteString("NAME                          STATUS      PLAN        REGION\n")
	sb.WriteString("─────────────────────────────────────────────────────────────\n")

	for _, site := range sites {
		sb.WriteString(fmt.Sprintf("%-29s %-11s %-11s %s\n",
			truncate(site.Name, 29),
			site.Status,
			site.Plan,
			site.Region,
		))
	}

	return sb.String()
}

// FormatBenchList formats benches for display
func FormatBenchList(benches []Bench) string {
	if len(benches) == 0 {
		return "No benches found."
	}

	var sb strings.Builder
	sb.WriteString("NAME                          VERSION     STATUS      APPS\n")
	sb.WriteString("─────────────────────────────────────────────────────────────\n")

	for _, bench := range benches {
		apps := strings.Join(bench.Apps, ", ")
		if len(apps) > 20 {
			apps = apps[:17] + "..."
		}
		sb.WriteString(fmt.Sprintf("%-29s %-11s %-11s %s\n",
			truncate(bench.Title, 29),
			bench.Version,
			bench.Status,
			apps,
		))
	}

	return sb.String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
