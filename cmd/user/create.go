package user

import (
	"fmt"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/api"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create <email>",
	Short: "Create a new user",
	Long: `Create a new user in the site.

Examples:
  weg user create test@example.com
  weg user create test@example.com --first-name Test --last-name User
  weg user create test@example.com --role "System Manager"`,
	Args: cobra.ExactArgs(1),
	RunE: runCreate,
}

var (
	createSite      string
	createFirstName string
	createLastName  string
	createRole      string
	createPassword  string
)

func init() {
	UserCmd.AddCommand(createCmd)
	createCmd.Flags().StringVarP(&createSite, "site", "s", "", "Site to create user in")
	createCmd.Flags().StringVar(&createFirstName, "first-name", "", "User's first name")
	createCmd.Flags().StringVar(&createLastName, "last-name", "", "User's last name")
	createCmd.Flags().StringVar(&createRole, "role", "", "Role to assign")
	createCmd.Flags().StringVar(&createPassword, "password", "", "Initial password")
}

func runCreate(cmd *cobra.Command, args []string) error {
	email := args[0]

	benchPath, site, err := resolveContext(createSite)
	if err != nil {
		return err
	}

	executor := api.NewExecutor(benchPath, site, "Administrator")

	firstName := createFirstName
	if firstName == "" {
		firstName = email
	}

	roleAssign := ""
	if createRole != "" {
		roleAssign = fmt.Sprintf(`
    user.add_roles('%s')`, createRole)
	}

	passwordSet := ""
	if createPassword != "" {
		passwordSet = fmt.Sprintf(`
    from frappe.utils.password import update_password
    update_password(user.name, '%s')`, createPassword)
	}

	script := fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()

try:
    user = frappe.get_doc({
        'doctype': 'User',
        'email': '%s',
        'first_name': '%s',
        'last_name': '%s',
        'enabled': 1,
        'user_type': 'System User',
        'send_welcome_email': 0
    })
    user.insert(ignore_permissions=True)
    %s
    %s
    frappe.db.commit()
    print(json.dumps({"success": True, "data": {"name": user.name, "email": user.email}}))
except Exception as ex:
    frappe.db.rollback()
    print(json.dumps({"success": False, "error": str(ex)}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site, email, firstName, createLastName, roleAssign, passwordSet)

	result, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("failed to create user: %s", result.Error)
	}

	fmt.Printf("Created user: %s\n", email)
	if createRole != "" {
		fmt.Printf("Assigned role: %s\n", createRole)
	}
	if createPassword == "" {
		fmt.Println("No password set. Use 'weg user password' to set one.")
	}

	return nil
}
