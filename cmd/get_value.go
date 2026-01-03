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

var getValueCmd = &cobra.Command{
	Use:   "get-value <doctype> <name> <field>",
	Short: "Get a field value from a document",
	Long: `Get a field value from a document directly.

This retrieves a single field without loading the full document.

Examples:
  weg get-value User Administrator email
  weg get-value "Sales Invoice" INV-001 grand_total
  weg get-value User test@test.com user_type`,
	Args: cobra.ExactArgs(3),
	RunE: runGetValue,
}

var getValueSite string

func init() {
	rootCmd.AddCommand(getValueCmd)
	getValueCmd.Flags().StringVar(&getValueSite, "site", "", "Site to query")
}

func runGetValue(cmd *cobra.Command, args []string) error {
	doctype := args[0]
	name := args[1]
	field := args[2]

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

	site := getValueSite
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

	executor := api.NewExecutor(benchPath, site, "Administrator")

	script := fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()

try:
    value = frappe.db.get_value('%s', '%s', '%s')
    print(json.dumps({"success": True, "data": value}, default=str))
except Exception as ex:
    import traceback
    print(json.dumps({"success": False, "error": str(ex), "traceback": traceback.format_exc()}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site, doctype, name, field)

	apiResult, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to get value: %w", err)
	}

	if !apiResult.Success {
		if apiResult.Traceback != "" {
			fmt.Fprintf(os.Stderr, "%s\n", apiResult.Traceback)
		}
		return fmt.Errorf("failed to get value: %s", apiResult.Error)
	}

	// Print the value
	switch v := apiResult.Data.(type) {
	case nil:
		fmt.Println("null")
	case string:
		fmt.Println(v)
	case float64:
		if v == float64(int(v)) {
			fmt.Printf("%.0f\n", v)
		} else {
			fmt.Printf("%v\n", v)
		}
	case bool:
		fmt.Printf("%v\n", v)
	default:
		output, _ := json.MarshalIndent(v, "", "  ")
		fmt.Println(string(output))
	}

	return nil
}
