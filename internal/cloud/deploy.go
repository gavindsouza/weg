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

// Job represents a Frappe Cloud job (deploy, migrate, backup, etc.)
type Job struct {
	Name     string `json:"name"`
	JobType  string `json:"job_type"`
	Status   string `json:"status"`
	Site     string `json:"site"`
	Bench    string `json:"bench"`
	Creation string `json:"creation"`
	Start    string `json:"start"`
	End      string `json:"end"`
	Duration string `json:"duration"`
}

// SiteDetail represents detailed site information
type SiteDetail struct {
	Name            string         `json:"name"`
	Host            string         `json:"host_name"`
	Status          string         `json:"status"`
	CurrentPlan     map[string]any `json:"current_plan"`
	Bench           string         `json:"bench"`
	BenchTitle      string         `json:"bench_title"`
	FrappeVersion   string         `json:"frappe_version"`
	CreatedBy       string         `json:"created_by"`
	CreatedAt       string         `json:"creation"`
	InstalledApps   []InstalledApp `json:"installed_apps,omitempty"`
	UpdateAvailable bool           `json:"update_available"`
}

// InstalledApp represents an app installed on a site
type InstalledApp struct {
	App    string `json:"app"`
	Title  string `json:"title"`
	Hash   string `json:"hash"`
	Branch string `json:"branch"`
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

// GetSiteDetail returns detailed site information
func (c *Client) GetSiteDetail(siteName string) (*SiteDetail, error) {
	params := url.Values{}
	params.Set("name", siteName)

	resp, err := c.doRequest("GET", "/method/press.api.site.get", params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %s", string(body))
	}

	var result struct {
		Message SiteDetail `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %s", string(body))
	}

	return &result.Message, nil
}

// GetSiteJobs returns jobs for a site
func (c *Client) GetSiteJobs(siteName string, limit int) ([]Job, error) {
	params := url.Values{}
	params.Set("filters", fmt.Sprintf(`{"site": "%s"}`, siteName))
	params.Set("order_by", "creation desc")
	if limit > 0 {
		params.Set("limit_page_length", fmt.Sprintf("%d", limit))
	}

	resp, err := c.doRequest("GET", "/method/press.api.site.jobs", params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %s", string(body))
	}

	var result struct {
		Message []Job `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %s", string(body))
	}

	return result.Message, nil
}

// GetRunningJobs returns currently running jobs for a site
func (c *Client) GetRunningJobs(siteName string) ([]Job, error) {
	params := url.Values{}
	params.Set("name", siteName)

	resp, err := c.doRequest("GET", "/method/press.api.site.running_jobs", params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %s", string(body))
	}

	var result struct {
		Message []Job `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %s", string(body))
	}

	return result.Message, nil
}

// GetJob returns details for a specific job
func (c *Client) GetJob(jobName string) (*Job, error) {
	params := url.Values{}
	params.Set("job", jobName)

	resp, err := c.doRequest("GET", "/method/press.api.site.job", params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %s", string(body))
	}

	var result struct {
		Message Job `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %s", string(body))
	}

	return &result.Message, nil
}

// GetBenchJobs returns jobs for a bench/release group
func (c *Client) GetBenchJobs(benchName string, limit int) ([]Job, error) {
	params := url.Values{}
	params.Set("filters", fmt.Sprintf(`{"group": "%s"}`, benchName))
	params.Set("order_by", "creation desc")
	if limit > 0 {
		params.Set("limit_page_length", fmt.Sprintf("%d", limit))
	}

	resp, err := c.doRequest("GET", "/method/press.api.bench.jobs", params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %s", string(body))
	}

	var result struct {
		Message []Job `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %s", string(body))
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
