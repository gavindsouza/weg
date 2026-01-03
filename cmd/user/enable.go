package user

import (
	"fmt"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/api"
	"github.com/spf13/cobra"
)

var enableCmd = &cobra.Command{
	Use:   "enable <email>",
	Short: "Enable a user",
	Long: `Enable a disabled user account.

Examples:
  weg user enable test@example.com`,
	Args: cobra.ExactArgs(1),
	RunE: runEnable,
}

var enableSite string

func init() {
	UserCmd.AddCommand(enableCmd)
	enableCmd.Flags().StringVarP(&enableSite, "site", "s", "", "Site to update")
}

func runEnable(cmd *cobra.Command, args []string) error {
	return setUserEnabled(args[0], enableSite, true)
}

var disableCmd = &cobra.Command{
	Use:   "disable <email>",
	Short: "Disable a user",
	Long: `Disable a user account. The user will not be able to log in.

Examples:
  weg user disable test@example.com`,
	Args: cobra.ExactArgs(1),
	RunE: runDisable,
}

var disableSite string

func init() {
	UserCmd.AddCommand(disableCmd)
	disableCmd.Flags().StringVarP(&disableSite, "site", "s", "", "Site to update")
}

func runDisable(cmd *cobra.Command, args []string) error {
	return setUserEnabled(args[0], disableSite, false)
}

func setUserEnabled(email, siteName string, enabled bool) error {
	benchPath, site, err := resolveContext(siteName)
	if err != nil {
		return err
	}

	executor := api.NewExecutor(benchPath, site, "Administrator")

	enabledVal := 0
	if enabled {
		enabledVal = 1
	}

	script := fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()

try:
    frappe.db.set_value('User', '%s', 'enabled', %d)
    frappe.db.commit()
    print(json.dumps({"success": True}))
except Exception as ex:
    frappe.db.rollback()
    print(json.dumps({"success": False, "error": str(ex)}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site, email, enabledVal)

	result, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("failed to update user: %s", result.Error)
	}

	action := "disabled"
	if enabled {
		action = "enabled"
	}
	fmt.Printf("User %s %s\n", email, action)

	return nil
}
