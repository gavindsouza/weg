package user

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/gavindsouza/weg/internal/api"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var passwordCmd = &cobra.Command{
	Use:   "password <email>",
	Short: "Set user password",
	Long: `Set or reset a user's password.

Examples:
  weg user password Administrator
  weg user password test@example.com --password secret123`,
	Args: cobra.ExactArgs(1),
	RunE: runPassword,
}

var (
	passwordSite   string
	passwordValue  string
	passwordLogout bool
)

func init() {
	UserCmd.AddCommand(passwordCmd)
	passwordCmd.Flags().StringVarP(&passwordSite, "site", "s", "", "Site to update")
	passwordCmd.Flags().StringVar(&passwordValue, "password", "", "New password (prompts if not provided)")
	passwordCmd.Flags().BoolVar(&passwordLogout, "logout", false, "Logout all sessions after password change")
}

func runPassword(cmd *cobra.Command, args []string) error {
	email := args[0]

	benchPath, site, err := resolveContext(passwordSite)
	if err != nil {
		return err
	}

	password := passwordValue
	if password == "" {
		password, err = promptPassword()
		if err != nil {
			return err
		}
	}

	executor := api.NewExecutor(benchPath, site, "Administrator")

	logoutCmd := ""
	if passwordLogout {
		logoutCmd = fmt.Sprintf(`
    frappe.db.delete('Sessions', filters={'user': '%s'})`, email)
	}

	script := fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()

try:
    from frappe.utils.password import update_password
    update_password('%s', '%s')
    %s
    frappe.db.commit()
    print(json.dumps({"success": True}))
except Exception as ex:
    frappe.db.rollback()
    print(json.dumps({"success": False, "error": str(ex)}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site, email, strings.ReplaceAll(password, "'", "\\'"), logoutCmd)

	result, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to set password: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("failed to set password: %s", result.Error)
	}

	fmt.Printf("Password updated for %s\n", email)
	if passwordLogout {
		fmt.Println("All sessions have been logged out")
	}

	return nil
}

func promptPassword() (string, error) {
	fmt.Print("New password: ")
	password, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		// Fallback for non-terminal input
		reader := bufio.NewReader(os.Stdin)
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("failed to read password: %w", err)
		}
		return strings.TrimSpace(line), nil
	}

	fmt.Print("Confirm password: ")
	confirm, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("failed to read confirmation: %w", err)
	}

	if string(password) != string(confirm) {
		return "", fmt.Errorf("passwords do not match")
	}

	return string(password), nil
}
