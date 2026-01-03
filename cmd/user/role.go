package user

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/api"
	"github.com/spf13/cobra"
)

var roleCmd = &cobra.Command{
	Use:   "role",
	Short: "Manage user roles",
	Long: `Add or remove roles from users.

Examples:
  weg user role add test@example.com "System Manager"
  weg user role remove test@example.com "System Manager"
  weg user role list test@example.com`,
}

var roleAddCmd = &cobra.Command{
	Use:   "add <email> <role>",
	Short: "Add a role to a user",
	Long: `Add a role to a user account.

Examples:
  weg user role add test@example.com "System Manager"
  weg user role add admin@example.com "Administrator"`,
	Args: cobra.ExactArgs(2),
	RunE: runRoleAdd,
}

var roleRemoveCmd = &cobra.Command{
	Use:   "remove <email> <role>",
	Short: "Remove a role from a user",
	Long: `Remove a role from a user account.

Examples:
  weg user role remove test@example.com "System Manager"
  weg user role remove admin@example.com "HR Manager"`,
	Args: cobra.ExactArgs(2),
	RunE: runRoleRemove,
}

var roleListCmd = &cobra.Command{
	Use:   "list <email>",
	Short: "List roles for a user",
	Long: `List all roles assigned to a user account.

Examples:
  weg user role list Administrator
  weg user role list test@example.com`,
	Args: cobra.ExactArgs(1),
	RunE: runRoleList,
}

var roleSite string

func init() {
	UserCmd.AddCommand(roleCmd)
	roleCmd.AddCommand(roleAddCmd)
	roleCmd.AddCommand(roleRemoveCmd)
	roleCmd.AddCommand(roleListCmd)
	roleCmd.PersistentFlags().StringVarP(&roleSite, "site", "s", "", "Site to query/update")
}

func runRoleAdd(cmd *cobra.Command, args []string) error {
	email := args[0]
	role := args[1]

	benchPath, site, err := resolveContext(roleSite)
	if err != nil {
		return err
	}

	executor := api.NewExecutor(benchPath, site, "Administrator")

	script := fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()

try:
    user = frappe.get_doc('User', '%s')
    user.add_roles('%s')
    frappe.db.commit()
    print(json.dumps({"success": True}))
except Exception as ex:
    frappe.db.rollback()
    print(json.dumps({"success": False, "error": str(ex)}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site, email, role)

	result, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to add role: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("failed to add role: %s", result.Error)
	}

	fmt.Printf("Added role '%s' to %s\n", role, email)
	return nil
}

func runRoleRemove(cmd *cobra.Command, args []string) error {
	email := args[0]
	role := args[1]

	benchPath, site, err := resolveContext(roleSite)
	if err != nil {
		return err
	}

	executor := api.NewExecutor(benchPath, site, "Administrator")

	script := fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()

try:
    user = frappe.get_doc('User', '%s')
    user.remove_roles('%s')
    frappe.db.commit()
    print(json.dumps({"success": True}))
except Exception as ex:
    frappe.db.rollback()
    print(json.dumps({"success": False, "error": str(ex)}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site, email, role)

	result, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to remove role: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("failed to remove role: %s", result.Error)
	}

	fmt.Printf("Removed role '%s' from %s\n", role, email)
	return nil
}

func runRoleList(cmd *cobra.Command, args []string) error {
	email := args[0]

	benchPath, site, err := resolveContext(roleSite)
	if err != nil {
		return err
	}

	executor := api.NewExecutor(benchPath, site, "Administrator")

	script := fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()

try:
    roles = frappe.get_all('Has Role',
        filters={'parent': '%s', 'parenttype': 'User'},
        pluck='role',
        order_by='role'
    )
    print(json.dumps({"success": True, "data": roles}))
except Exception as ex:
    print(json.dumps({"success": False, "error": str(ex)}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site, email)

	result, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to list roles: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("failed to list roles: %s", result.Error)
	}

	roles, ok := result.Data.([]interface{})
	if !ok || len(roles) == 0 {
		fmt.Printf("No roles assigned to %s\n", email)
		return nil
	}

	fmt.Printf("Roles for %s:\n", email)
	for _, r := range roles {
		fmt.Printf("  - %s\n", r.(string))
	}

	return nil
}

var showCmd = &cobra.Command{
	Use:   "show <email>",
	Short: "Show user details",
	Long: `Display detailed information about a user account.

Shows the user's profile, status, last login, and assigned roles.

Examples:
  weg user show Administrator
  weg user show test@example.com`,
	Args: cobra.ExactArgs(1),
	RunE: runShow,
}

var showSite string

func init() {
	UserCmd.AddCommand(showCmd)
	showCmd.Flags().StringVarP(&showSite, "site", "s", "", "Site to query")
}

func runShow(cmd *cobra.Command, args []string) error {
	email := args[0]

	benchPath, site, err := resolveContext(showSite)
	if err != nil {
		return err
	}

	executor := api.NewExecutor(benchPath, site, "Administrator")

	script := fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()

try:
    user = frappe.get_doc('User', '%s')
    roles = [r.role for r in user.roles]
    data = {
        'email': user.email,
        'first_name': user.first_name,
        'last_name': user.last_name,
        'full_name': user.full_name,
        'enabled': user.enabled,
        'user_type': user.user_type,
        'last_login': user.last_login,
        'last_active': user.last_active,
        'creation': user.creation,
        'roles': roles
    }
    print(json.dumps({"success": True, "data": data}, default=str))
except Exception as ex:
    print(json.dumps({"success": False, "error": str(ex)}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site, email)

	result, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("failed to get user: %s", result.Error)
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("unexpected response format")
	}

	fmt.Printf("User: %s\n", email)
	fmt.Println(strings.Repeat("-", 50))
	fmt.Printf("Name:       %s %s\n", getString(data, "first_name"), getString(data, "last_name"))
	fmt.Printf("Full Name:  %s\n", getString(data, "full_name"))
	fmt.Printf("Enabled:    %v\n", getBool(data, "enabled"))
	fmt.Printf("User Type:  %s\n", getString(data, "user_type"))
	fmt.Printf("Created:    %s\n", getString(data, "creation"))
	fmt.Printf("Last Login: %s\n", getString(data, "last_login"))
	fmt.Printf("Last Active:%s\n", getString(data, "last_active"))

	if roles, ok := data["roles"].([]interface{}); ok && len(roles) > 0 {
		fmt.Printf("\nRoles:\n")
		for _, r := range roles {
			fmt.Printf("  - %s\n", r.(string))
		}
	}

	return nil
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(float64); ok {
		return v == 1
	}
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}
