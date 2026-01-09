package cloud

import (
	"fmt"
	"os"
	"text/tabwriter"

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
		fmt.Println("No marketplace apps published yet.")
		fmt.Println("\nPublish an app at https://cloud.frappe.io/dashboard/marketplace")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "APP\tTITLE\tSTATUS")
	for _, app := range apps {
		fmt.Fprintf(w, "%s\t%s\t%s\n",
			app.Name,
			app.Title,
			app.Status,
		)
	}
	w.Flush()

	fmt.Printf("\nTotal: %d app(s)\n", len(apps))
	fmt.Println("\nUse 'weg cloud mp app <name>' for details and analytics")

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

	fmt.Printf("App: %s\n", app.Name)
	fmt.Printf("Title: %s\n", app.Title)
	fmt.Printf("Status: %s\n", app.Status)
	if app.Description != "" {
		fmt.Printf("Description: %s\n", app.Description)
	}

	// Show versions
	if len(app.Sources) > 0 {
		fmt.Println("\nSupported Versions:")
		for _, src := range app.Sources {
			fmt.Printf("  - %s\n", src.Version)
		}
	}

	// Get analytics
	analytics, err := client.GetAppAnalytics(appName)
	if err != nil {
		fmt.Printf("\nAnalytics: (unavailable)\n")
	} else {
		fmt.Println("\nAnalytics:")
		fmt.Printf("  Total Installs: %d\n", analytics.TotalInstalls)
		fmt.Printf("  Active Installs: %d\n", analytics.TotalActiveInstalls)
		if analytics.RevenueData != nil {
			fmt.Printf("  Total Revenue: %.2f %s\n", analytics.RevenueData.TotalRevenue, analytics.RevenueData.Currency)
			fmt.Printf("  Monthly Revenue: %.2f %s\n", analytics.RevenueData.MonthlyRevenue, analytics.RevenueData.Currency)
		}
	}

	fmt.Println("\nUse 'weg cloud mp subs", appName, "' for subscription details")

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
		fmt.Printf("No active subscriptions for %s\n", appName)
		return nil
	}

	fmt.Printf("Subscriptions for %s\n\n", appName)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SITE\tPLAN\tPRICE (USD)\tACTIVE DAYS\tSTATUS")

	var totalRevenue float64
	var activeCount int

	for _, sub := range subs {
		status := "inactive"
		if sub.Enabled == 1 {
			status = "active"
			activeCount++
		}
		fmt.Fprintf(w, "%s\t%s\t$%.2f\t%d\t%s\n",
			sub.Site,
			sub.AppPlan,
			sub.PriceUSD,
			sub.ActiveDays,
			status,
		)
		if sub.Enabled == 1 {
			totalRevenue += sub.PriceUSD * float64(sub.ActiveDays) / 30 // rough monthly estimate
		}
	}
	w.Flush()

	fmt.Printf("\nTotal: %d subscription(s), %d active\n", len(subs), activeCount)

	return nil
}
