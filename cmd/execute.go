package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/api"
	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var executeCmd = &cobra.Command{
	Use:   "execute <method> [args...]",
	Short: "Execute a Frappe method",
	Long: `Execute a Frappe method directly.

Methods can be any Python function that's accessible from Frappe.
Arguments are passed as keyword arguments.

Examples:
  weg execute frappe.get_hooks
  weg execute frappe.client.get --doctype User --name Administrator
  weg execute myapp.api.create_order --customer CUST-001 --items '[{"item": "X"}]'
  weg execute frappe.utils.now --site test.localhost`,
	Args: cobra.MinimumNArgs(1),
	RunE: runExecute,
}

var (
	executeSite string
	executeUser string
	executeRaw  bool
)

func init() {
	rootCmd.AddCommand(executeCmd)
	executeCmd.Flags().StringVar(&executeSite, "site", "", "Site to execute on")
	executeCmd.Flags().StringVar(&executeUser, "user", "Administrator", "User context for execution")
	executeCmd.Flags().BoolVar(&executeRaw, "raw", false, "Output raw result without formatting")
}

func runExecute(cmd *cobra.Command, args []string) error {
	method := args[0]

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
	site := executeSite
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
		return fmt.Errorf("no site specified and no default site found")
	}

	// Parse remaining args as keyword arguments
	kwargs := make(map[string]interface{})
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--") {
			key := strings.TrimPrefix(arg, "--")
			key = strings.ReplaceAll(key, "-", "_")

			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				value := args[i+1]
				// Try to parse as JSON if it looks like JSON
				if (strings.HasPrefix(value, "{") || strings.HasPrefix(value, "[")) {
					var jsonVal interface{}
					if err := json.Unmarshal([]byte(value), &jsonVal); err == nil {
						kwargs[key] = jsonVal
						i++
						continue
					}
				}
				// Try to parse as number
				if value == "true" {
					kwargs[key] = true
				} else if value == "false" {
					kwargs[key] = false
				} else {
					kwargs[key] = value
				}
				i++
			} else {
				// Flag without value, treat as true
				kwargs[key] = true
			}
		}
	}

	executor := api.NewExecutor(benchPath, site, executeUser)

	// Build kwargs JSON
	kwargsJSON, err := json.Marshal(kwargs)
	if err != nil {
		return fmt.Errorf("failed to serialize arguments: %w", err)
	}

	script := fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()
frappe.set_user('%s')

try:
    kwargs = json.loads('''%s''')

    # Import and call the method
    method_path = '%s'
    parts = method_path.rsplit('.', 1)

    if len(parts) == 2:
        module_path, func_name = parts
        module = frappe.get_module(module_path)
        func = getattr(module, func_name)
    else:
        func = getattr(frappe, parts[0])

    result = func(**kwargs)
    print(json.dumps({"success": True, "data": result}, default=str, indent=2))
except Exception as ex:
    import traceback
    print(json.dumps({
        "success": False,
        "error": str(ex),
        "traceback": traceback.format_exc()
    }))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site, executeUser, kwargsJSON, method)

	apiResult, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to execute method: %w", err)
	}

	if !apiResult.Success {
		if apiResult.Traceback != "" {
			fmt.Fprintf(os.Stderr, "Traceback:\n%s\n", apiResult.Traceback)
		}
		return fmt.Errorf("execution failed: %s", apiResult.Error)
	}

	// Output result
	if executeRaw {
		output, err := json.Marshal(apiResult.Data)
		if err != nil {
			return fmt.Errorf("failed to serialize result: %w", err)
		}
		fmt.Println(string(output))
	} else {
		output, err := json.MarshalIndent(apiResult.Data, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to serialize result: %w", err)
		}
		fmt.Println(string(output))
	}

	return nil
}
