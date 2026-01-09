package cloud

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// PublisherProfile represents a marketplace publisher profile
type PublisherProfile struct {
	ProfileCreated bool                   `json:"profile_created"`
	ProfileInfo    *PublisherProfileInfo  `json:"profile_info,omitempty"`
}

// PublisherProfileInfo contains publisher details
type PublisherProfileInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	ContactEmail string `json:"contact_email"`
}

// MarketplaceApp represents a published marketplace app
type MarketplaceApp struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	App         string `json:"app"`
	Image       string `json:"image"`
	Status      string `json:"status"`
	Description string `json:"description"`
}

// MarketplaceAppDetail represents detailed app info
type MarketplaceAppDetail struct {
	Name            string                  `json:"name"`
	Title           string                  `json:"title"`
	App             string                  `json:"app"`
	Image           string                  `json:"image"`
	Status          string                  `json:"status"`
	Description     string                  `json:"description"`
	LongDescription string                  `json:"long_description"`
	Team            string                  `json:"team"`
	Sources         []MarketplaceAppSource  `json:"sources"`
}

// MarketplaceAppSource represents a version source
type MarketplaceAppSource struct {
	Version string `json:"version"`
	Source  string `json:"source"`
}

// AppAnalytics represents analytics data for an app
type AppAnalytics struct {
	TotalInstalls      int                `json:"total_installs"`
	TotalActiveInstalls int               `json:"total_active_installs"`
	InstallsByPlan     []PlanInstalls     `json:"installs_by_plan,omitempty"`
	RevenueData        *RevenueData       `json:"revenue_data,omitempty"`
}

// PlanInstalls represents installs per plan
type PlanInstalls struct {
	Plan   string `json:"plan"`
	Count  int    `json:"count"`
}

// RevenueData represents revenue information
type RevenueData struct {
	TotalRevenue   float64 `json:"total_revenue"`
	MonthlyRevenue float64 `json:"monthly_revenue"`
	Currency       string  `json:"currency"`
}

// AppSubscription represents a subscription to an app
type AppSubscription struct {
	Site        string  `json:"site"`
	UserContact string  `json:"user_contact"`
	AppPlan     string  `json:"app_plan"`
	PriceUSD    float64 `json:"price_usd"`
	PriceINR    float64 `json:"price_inr"`
	Enabled     int     `json:"enabled"`
	ActiveDays  int     `json:"active_days"`
}

// GetPublisherProfile returns the current user's publisher profile
func (c *Client) GetPublisherProfile() (*PublisherProfile, error) {
	resp, err := c.doRequest("GET", "/method/press.api.marketplace.get_publisher_profile_info", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Message PublisherProfile `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %s", string(body))
	}

	return &result.Message, nil
}

// GetMarketplaceApps returns all marketplace apps for the current team
func (c *Client) GetMarketplaceApps() ([]MarketplaceApp, error) {
	resp, err := c.doRequest("GET", "/method/press.api.marketplace.get_apps", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Message []MarketplaceApp `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %s", string(body))
	}

	return result.Message, nil
}

// GetMarketplaceApp returns details for a specific marketplace app
func (c *Client) GetMarketplaceApp(appName string) (*MarketplaceAppDetail, error) {
	params := url.Values{}
	params.Set("name", appName)

	resp, err := c.doRequest("GET", "/method/press.api.marketplace.get_app", params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Message MarketplaceAppDetail `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %s", string(body))
	}

	return &result.Message, nil
}

// GetAppAnalytics returns analytics for a marketplace app
func (c *Client) GetAppAnalytics(appName string) (*AppAnalytics, error) {
	params := url.Values{}
	params.Set("name", appName)

	resp, err := c.doRequest("GET", "/method/press.api.marketplace.analytics", params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Message AppAnalytics `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %s", string(body))
	}

	return &result.Message, nil
}

// GetAppSubscriptions returns subscriptions for a marketplace app
func (c *Client) GetAppSubscriptions(appName string) ([]AppSubscription, error) {
	params := url.Values{}
	params.Set("marketplace_app", appName)

	resp, err := c.doRequest("GET", "/method/press.api.marketplace.get_subscriptions_list", params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Message []AppSubscription `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %s", string(body))
	}

	return result.Message, nil
}
