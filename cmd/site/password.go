package site

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/gavindsouza/weg/internal/api"
	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var passwordCmd = &cobra.Command{
	Use:   "password [email]",
	Short: "Set or reset a user password",
	Long: `Set or reset a user's password.

If no email is provided, defaults to Administrator.
If no password is provided via --password flag, prompts for it interactively.

This is commonly used after:
  - Restoring from a production backup
  - Setting up a new development environment
  - Resetting a forgotten password

Examples:
  weg site password                      # Reset Administrator password (prompts)
  weg site password admin@example.com    # Reset specific user password
  weg site password --site test          # Reset password for specific site
  weg site password --password secret    # Non-interactive (not recommended)`,
	Args: cobra.MaximumNArgs(1),
	RunE: runPassword,
}

var (
	passwordSite   string
	passwordValue  string
	passwordLogout bool
)

func init() {
	SiteCmd.AddCommand(passwordCmd)
	passwordCmd.Flags().StringVar(&passwordSite, "site", "", "Site to update password for")
	passwordCmd.Flags().StringVar(&passwordValue, "password", "", "New password (prompts if not provided)")
	passwordCmd.Flags().BoolVar(&passwordLogout, "logout-all-sessions", false, "Logout all existing sessions")
}

func runPassword(cmd *cobra.Command, args []string) error {
	path := "."
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	result, err := config.DetectContext(absPath)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	var benchPath string
	switch result.Context {
	case config.ContextWegBench:
		benchPath = absPath
	case config.ContextWegApp:
		benchPath = filepath.Join(absPath, ".weg")
	default:
		return fmt.Errorf("not a weg-managed project")
	}

	// Determine site
	site := passwordSite
	if site == "" {
		st, err := state.Load(absPath)
		if err == nil {
			site = st.GetDefaultSite()
		}
		if site == "" {
			// Try currentsite.txt
			currentSitePath := filepath.Join(benchPath, "sites", "currentsite.txt")
			data, err := os.ReadFile(currentSitePath)
			if err == nil {
				site = strings.TrimSpace(string(data))
			}
		}
	}

	if site == "" {
		return fmt.Errorf("no site specified and no default site found. Use --site flag")
	}

	// Determine user
	user := "Administrator"
	if len(args) > 0 {
		user = args[0]
	}

	// Get password
	password := passwordValue
	if password == "" {
		password, err = promptPassword(user)
		if err != nil {
			return err
		}
	}

	if len(password) < 4 {
		return fmt.Errorf("password must be at least 4 characters")
	}

	fmt.Printf("Setting password for %s on %s...\n", user, site)

	// Execute password update
	executor := api.NewExecutor(benchPath, site, "Administrator")

	// Build the script to update password
	script := fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()

try:
    from frappe.utils.password import update_password
    update_password('%s', '%s', logout_all_sessions=%s)
    frappe.db.commit()
    print(json.dumps({"success": True, "data": "password updated"}))
except Exception as ex:
    frappe.db.rollback()
    print(json.dumps({"success": False, "error": str(ex)}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site, user, escapeForPython(password), pythonBool(passwordLogout))

	apiResult, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to set password: %w", err)
	}

	if !apiResult.Success {
		return fmt.Errorf("failed to set password: %s", apiResult.Error)
	}

	fmt.Printf("Password updated for %s\n", user)
	if passwordLogout {
		fmt.Println("All existing sessions have been logged out")
	}
	return nil
}

func promptPassword(user string) (string, error) {
	fmt.Printf("New password for %s: ", user)

	// Try to read password without echo (works on terminal)
	if term.IsTerminal(int(syscall.Stdin)) {
		password, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println() // Add newline after password entry
		if err != nil {
			return "", fmt.Errorf("failed to read password: %w", err)
		}

		// Confirm password
		fmt.Printf("Confirm password: ")
		confirm, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return "", fmt.Errorf("failed to read password confirmation: %w", err)
		}

		if string(password) != string(confirm) {
			return "", fmt.Errorf("passwords do not match")
		}

		return string(password), nil
	}

	// Fallback for non-terminal (piped input)
	reader := bufio.NewReader(os.Stdin)
	password, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read password: %w", err)
	}
	return strings.TrimSpace(password), nil
}

func escapeForPython(s string) string {
	// Escape single quotes and backslashes for Python string
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	return s
}

func pythonBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}
