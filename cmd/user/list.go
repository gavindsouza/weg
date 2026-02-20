package user

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/api"
	"github.com/gavindsouza/weg/internal/config"
	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all users",
	Long: `List all users in the site.

Examples:
  weg user list
  weg user list --enabled
  weg user list --role "System Manager"`,
	RunE: runList,
}

var (
	listSite    string
	listEnabled bool
	listRole    string
)

func init() {
	UserCmd.AddCommand(listCmd)
	listCmd.Flags().StringVarP(&listSite, "site", "s", "", "Site to query")
	listCmd.Flags().BoolVar(&listEnabled, "enabled", false, "Only show enabled users")
	listCmd.Flags().StringVar(&listRole, "role", "", "Filter by role")
}

func runList(cmd *cobra.Command, args []string) error {
	benchPath, site, err := resolveContext(listSite)
	if err != nil {
		return err
	}

	executor := api.NewExecutor(benchPath, site, "Administrator")

	filters := ""
	if listEnabled {
		filters = ", 'enabled': 1"
	}

	roleFilter := ""
	if listRole != "" {
		roleFilter = fmt.Sprintf(`
    role_users = frappe.get_all('Has Role', filters={'role': '%s', 'parenttype': 'User'}, pluck='parent')
    filters['name'] = ['in', role_users]`, listRole)
	}

	script := fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()

try:
    filters = {'user_type': 'System User'%s}
    %s
    users = frappe.get_all('User',
        filters=filters,
        fields=['name', 'full_name', 'enabled', 'last_login'],
        order_by='name'
    )
    print(json.dumps({"success": True, "data": users}, default=str))
except Exception as ex:
    print(json.dumps({"success": False, "error": str(ex)}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site, filters, roleFilter)

	result, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to list users: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("failed to list users: %s", result.Error)
	}

	users, ok := result.Data.([]interface{})
	if !ok || len(users) == 0 {
		fmt.Println("No users found")
		return nil
	}

	fmt.Printf("%-35s %-25s %-8s %s\n", "EMAIL", "NAME", "ENABLED", "LAST LOGIN")
	fmt.Println(strings.Repeat("-", 90))
	for _, u := range users {
		user := u.(map[string]interface{})
		email := user["name"].(string)
		fullName := ""
		if fn, ok := user["full_name"].(string); ok {
			fullName = fn
		}
		enabled := "No"
		if e, ok := user["enabled"].(float64); ok && e == 1 {
			enabled = "Yes"
		}
		lastLogin := ""
		if ll, ok := user["last_login"].(string); ok {
			lastLogin = ll
		}
		fmt.Printf("%-35s %-25s %-8s %s\n", email, truncate(fullName, 25), enabled, lastLogin)
	}

	return nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func resolveContext(siteName string) (string, string, error) {
	path := "."
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", "", fmt.Errorf("invalid path: %w", err)
	}

	result, err := config.DetectContext(absPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to detect context: %w", err)
	}

	var benchPath string
	switch result.Context {
	case config.ContextWegBench:
		benchPath = absPath
	case config.ContextWegApp:
		benchPath = filepath.Join(absPath, ".weg")
	default:
		return "", "", wegerrors.NotInProject(absPath)
	}

	site := siteName
	if site == "" {
		st, err := state.Load(absPath)
		if err == nil {
			site = st.GetDefaultSite()
		}
		if site == "" {
			currentSitePath := filepath.Join(benchPath, "sites", "currentsite.txt")
			data, _ := os.ReadFile(currentSitePath)
			site = strings.TrimSpace(string(data))
		}
	}

	if site == "" {
		return "", "", fmt.Errorf("no site specified and no default site found")
	}

	return benchPath, site, nil
}
