package doctype

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/api"
	"github.com/gavindsouza/weg/internal/completion"
	wegerrors "github.com/gavindsouza/weg/internal/errors"
	wegoutput "github.com/gavindsouza/weg/internal/output"
	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show <doctype>",
	Short: "Show DocType structure",
	Long: `Show the fields and structure of a DocType.

Examples:
  weg doctype show User
  weg doctype show "Sales Invoice"
  weg doctype show MyDocType --json`,
	Args:              cobra.ExactArgs(1),
	RunE:              runShow,
	ValidArgsFunction: completion.CompleteDocTypesForArg(0),
}

var (
	showSite string
	showJSON bool
)

func init() {
	DoctypeCmd.AddCommand(showCmd)
	showCmd.Flags().StringVarP(&showSite, "site", "s", "", "Site to query")
	showCmd.Flags().BoolVar(&showJSON, "json", false, "Output as JSON")
}

func runShow(cmd *cobra.Command, args []string) error {
	doctype := args[0]

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
    meta = frappe.get_meta('%s')
    fields = []
    for f in meta.fields:
        fields.append({
            'fieldname': f.fieldname,
            'fieldtype': f.fieldtype,
            'label': f.label,
            'reqd': f.reqd,
            'options': f.options if f.fieldtype in ['Link', 'Select', 'Table', 'Table MultiSelect', 'Dynamic Link'] else None
        })
    result = {
        'name': meta.name,
        'module': meta.module,
        'is_submittable': meta.is_submittable,
        'is_tree': meta.is_tree,
        'is_single': meta.issingle,
        'fields': fields
    }
    print(json.dumps({"success": True, "data": result}))
except Exception as ex:
    import traceback
    print(json.dumps({"success": False, "error": str(ex), "traceback": traceback.format_exc()}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site, doctype)

	result, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to get doctype: %w", err)
	}

	if !result.Success {
		if result.Traceback != "" {
			wegoutput.Errorf("%s", result.Traceback)
		}
		return fmt.Errorf("failed to get doctype: %s", result.Error)
	}

	if showJSON {
		output, _ := json.MarshalIndent(result.Data, "", "  ")
		wegoutput.Print(string(output))
		return nil
	}

	data, ok := result.Data.(map[string]any)
	if !ok {
		return wegerrors.Validation("response", "unexpected format")
	}

	wegoutput.Printf("DocType: %s", data["name"])
	wegoutput.Printf("Module: %s", data["module"])
	if sub, ok := data["is_submittable"].(float64); ok && sub == 1 {
		wegoutput.Print("Submittable: Yes")
	}
	if tree, ok := data["is_tree"].(float64); ok && tree == 1 {
		wegoutput.Print("Tree: Yes")
	}
	if single, ok := data["is_single"].(float64); ok && single == 1 {
		wegoutput.Print("Single: Yes")
	}
	wegoutput.Print("")
	wegoutput.Printf("%-25s %-15s %-6s %s", "FIELD", "TYPE", "REQD", "OPTIONS")
	wegoutput.Print(strings.Repeat("-", 70))

	fields, _ := data["fields"].([]any)
	for _, f := range fields {
		field := f.(map[string]any)
		fieldname := ""
		if fn, ok := field["fieldname"].(string); ok {
			fieldname = fn
		}
		fieldtype := ""
		if ft, ok := field["fieldtype"].(string); ok {
			fieldtype = ft
		}
		reqd := ""
		if r, ok := field["reqd"].(float64); ok && r == 1 {
			reqd = "*"
		}
		options := ""
		if o, ok := field["options"].(string); ok {
			options = o
		}

		// Truncate long options
		if len(options) > 25 {
			options = options[:22] + "..."
		}

		wegoutput.Printf("%-25s %-15s %-6s %s", fieldname, fieldtype, reqd, options)
	}

	return nil
}
