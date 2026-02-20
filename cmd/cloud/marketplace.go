package cloud

import (
	"fmt"

	"github.com/gavindsouza/weg/internal/output"
	"github.com/spf13/cobra"
)

var marketplaceCmd = &cobra.Command{
	Use:     "marketplace",
	Aliases: []string{"mp"},
	Short:   "Manage your Frappe Cloud marketplace apps",
	Long: `View and manage your marketplace apps on Frappe Cloud.

Shows your published apps, subscriptions, and analytics.

Examples:
  weg cloud marketplace              # List your marketplace apps
  weg cloud marketplace apps         # List your published apps
  weg cloud mp app <name>            # Details & analytics for an app
  weg cloud mp subs <name>           # Subscriptions for an app`,
}

var mpAppsCmd = &cobra.Command{
	Use:   "apps",
	Short: "List your marketplace apps",
	RunE:  runMpApps,
}

var mpAppCmd = &cobra.Command{
	Use:   "app <app-name>",
	Short: "Show details and analytics for a specific app",
	Args:  cobra.ExactArgs(1),
	RunE:  runMpApp,
}

var mpSubsCmd = &cobra.Command{
	Use:   "subs <app-name>",
	Short: "Show subscriptions for a specific app",
	Args:  cobra.ExactArgs(1),
	RunE:  runMpSubs,
}

func init() {
	marketplaceCmd.AddCommand(mpAppsCmd)
	marketplaceCmd.AddCommand(mpAppCmd)
	marketplaceCmd.AddCommand(mpSubsCmd)

	// Default action shows apps list
	marketplaceCmd.RunE = runMpApps
}

func runMpApps(cmd *cobra.Command, args []string) error {
	client, err := getAuthenticatedClient("")
	if err != nil {
		return err
	}

	apps, err := client.GetMarketplaceApps()
	if err != nil {
		return fmt.Errorf("failed to get marketplace apps: %w", err)
	}

	if len(apps) == 0 {
		output.Print("No marketplace apps published yet.")
		output.Print("\nPublish an app at https://cloud.frappe.io/dashboard/marketplace")
		return nil
	}

	type AppRow struct {
		App    string `json:"app"`
		Title  string `json:"title"`
		Status string `json:"status"`
	}

	var rows []AppRow
	for _, app := range apps {
		rows = append(rows, AppRow{
			App:    app.Name,
			Title:  app.Title,
			Status: app.Status,
		})
	}

	if err := output.List(rows); err != nil {
		return err
	}

	if output.EffectiveFormat() != output.FormatJSON {
		output.Printf("\nTotal: %d app(s)", len(apps))
		output.Print("\nUse 'weg cloud mp app <name>' for details and analytics")
	}

	return nil
}

func runMpApp(cmd *cobra.Command, args []string) error {
	appName := args[0]

	client, err := getAuthenticatedClient("")
	if err != nil {
		return err
	}

	// Get app details
	app, err := client.GetMarketplaceApp(appName)
	if err != nil {
		return fmt.Errorf("failed to get app details: %w", err)
	}

	output.Printf("App: %s", app.Name)
	output.Printf("Title: %s", app.Title)
	output.Printf("Status: %s", app.Status)
	if app.Description != "" {
		output.Printf("Description: %s", app.Description)
	}

	// Show versions
	if len(app.Sources) > 0 {
		output.Print("\nSupported Versions:")
		for _, src := range app.Sources {
			output.Printf("  - %s", src.Version)
		}
	}

	// Get analytics
	analytics, err := client.GetAppAnalytics(appName)
	if err != nil {
		output.Print("\nAnalytics: (unavailable)")
	} else {
		output.Print("\nAnalytics:")
		output.Printf("  Total Installs: %d", analytics.TotalInstalls)
		output.Printf("  Active Installs: %d", analytics.TotalActiveInstalls)
		if analytics.RevenueData != nil {
			output.Printf("  Total Revenue: %.2f %s", analytics.RevenueData.TotalRevenue, analytics.RevenueData.Currency)
			output.Printf("  Monthly Revenue: %.2f %s", analytics.RevenueData.MonthlyRevenue, analytics.RevenueData.Currency)
		}
	}

	output.Printf("\nUse 'weg cloud mp subs %s' for subscription details", appName)

	return nil
}

func runMpSubs(cmd *cobra.Command, args []string) error {
	appName := args[0]

	client, err := getAuthenticatedClient("")
	if err != nil {
		return err
	}

	subs, err := client.GetAppSubscriptions(appName)
	if err != nil {
		return fmt.Errorf("failed to get subscriptions: %w", err)
	}

	if len(subs) == 0 {
		output.Printf("No active subscriptions for %s", appName)
		return nil
	}

	if output.EffectiveFormat() != output.FormatJSON {
		output.Printf("Subscriptions for %s", appName)
	}

	type SubRow struct {
		Site       string  `json:"site"`
		Plan       string  `json:"plan"`
		PriceUSD   float64 `json:"price_usd"`
		ActiveDays int     `json:"active_days"`
		Status     string  `json:"status"`
	}

	var rows []SubRow
	var activeCount int

	for _, sub := range subs {
		status := "inactive"
		if sub.Enabled == 1 {
			status = "active"
			activeCount++
		}
		rows = append(rows, SubRow{
			Site:       sub.Site,
			Plan:       sub.AppPlan,
			PriceUSD:   sub.PriceUSD,
			ActiveDays: sub.ActiveDays,
			Status:     status,
		})
	}

	if err := output.List(rows); err != nil {
		return err
	}

	if output.EffectiveFormat() != output.FormatJSON {
		output.Printf("\nTotal: %d subscription(s), %d active", len(subs), activeCount)
	}

	return nil
}
