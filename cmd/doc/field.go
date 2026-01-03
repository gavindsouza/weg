package doc

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gavindsouza/weg/internal/api"
	"github.com/spf13/cobra"
)

var fieldCmd = &cobra.Command{
	Use:   "field",
	Short: "Get or set document field values",
	Long: `Quick access to individual field values.

Use this for fast operations on single fields without fetching
the entire document.

Examples:
  weg doc field get User Administrator email
  weg doc field set User Administrator enabled 0
  weg doc field get "Sales Invoice" INV-001 grand_total`,
}

var fieldGetCmd = &cobra.Command{
	Use:   "get <doctype> <name> <field>",
	Short: "Get a field value",
	Long: `Get the value of a specific field from a document.

Examples:
  weg doc field get User Administrator email
  weg doc field get Item ITEM-001 stock_uom
  weg doc field get "Sales Invoice" INV-001 grand_total`,
	Args: cobra.ExactArgs(3),
	RunE: runFieldGet,
}

var fieldSetCmd = &cobra.Command{
	Use:   "set <doctype> <name> <field> <value>",
	Short: "Set a field value",
	Long: `Set the value of a specific field on a document.

Supports various value types: strings, numbers, booleans (true/false), null.

Examples:
  weg doc field set User test@example.com enabled 0
  weg doc field set Item ITEM-001 disabled true
  weg doc field set Customer CUST-001 credit_limit 50000`,
	Args: cobra.ExactArgs(4),
	RunE: runFieldSet,
}

var fieldSite string

func init() {
	DocCmd.AddCommand(fieldCmd)
	fieldCmd.AddCommand(fieldGetCmd)
	fieldCmd.AddCommand(fieldSetCmd)

	fieldCmd.PersistentFlags().StringVarP(&fieldSite, "site", "s", "", "Site to query/update")
}

func runFieldGet(cmd *cobra.Command, args []string) error {
	doctype := args[0]
	name := args[1]
	field := args[2]

	benchPath, site, err := resolveContext(fieldSite)
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
    value = frappe.db.get_value('%s', '%s', '%s')
    print(json.dumps({"success": True, "data": value}, default=str))
except Exception as ex:
    print(json.dumps({"success": False, "error": str(ex)}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site, doctype, name, field)

	result, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to get value: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("failed to get value: %s", result.Error)
	}

	switch v := result.Data.(type) {
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

func runFieldSet(cmd *cobra.Command, args []string) error {
	doctype := args[0]
	name := args[1]
	field := args[2]
	valueStr := args[3]

	benchPath, site, err := resolveContext(fieldSite)
	if err != nil {
		return err
	}

	// Parse value for Python
	var valuePython string
	if valueStr == "true" || valueStr == "True" {
		valuePython = "True"
	} else if valueStr == "false" || valueStr == "False" {
		valuePython = "False"
	} else if valueStr == "null" || valueStr == "None" {
		valuePython = "None"
	} else if i, err := strconv.Atoi(valueStr); err == nil {
		valuePython = fmt.Sprintf("%d", i)
	} else if f, err := strconv.ParseFloat(valueStr, 64); err == nil {
		valuePython = fmt.Sprintf("%f", f)
	} else {
		valuePython = fmt.Sprintf("'%s'", strings.ReplaceAll(valueStr, "'", "\\'"))
	}

	executor := api.NewExecutor(benchPath, site, "Administrator")

	script := fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()

try:
    frappe.db.set_value('%s', '%s', '%s', %s)
    frappe.db.commit()
    print(json.dumps({"success": True}))
except Exception as ex:
    frappe.db.rollback()
    print(json.dumps({"success": False, "error": str(ex)}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site, doctype, name, field, valuePython)

	result, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to set value: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("failed to set value: %s", result.Error)
	}

	fmt.Printf("Set %s.%s = %s\n", doctype, field, valueStr)
	return nil
}
