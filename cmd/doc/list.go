package doc

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/api"
	"github.com/gavindsouza/weg/internal/completion"
	wegoutput "github.com/gavindsouza/weg/internal/output"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list <doctype>",
	Short: "List documents",
	Long: `List documents of a given doctype.

Examples:
  weg doc list User
  weg doc list User --limit 50
  weg doc list User -F '{"enabled":1}'
  weg doc list User --fields '["name","email","enabled"]'`,
	Args:              cobra.ExactArgs(1),
	RunE:              runList,
	ValidArgsFunction: completion.CompleteDocTypesForArg(0),
}

var (
	listSite    string
	listFilters string
	listFields  string
	listLimit   int
	listOrderBy string
)

func init() {
	DocCmd.AddCommand(listCmd)
	listCmd.Flags().StringVarP(&listSite, "site", "s", "", "Site to query")
	listCmd.Flags().StringVarP(&listFilters, "filters", "F", "", "JSON filter object")
	listCmd.Flags().StringVar(&listFields, "fields", "", "JSON array of fields")
	listCmd.Flags().IntVarP(&listLimit, "limit", "l", 20, "Limit results")
	listCmd.Flags().StringVar(&listOrderBy, "order-by", "", "Order by field")
}

func runList(cmd *cobra.Command, args []string) error {
	doctype := args[0]

	benchPath, site, err := resolveContext(listSite)
	if err != nil {
		return err
	}

	// Default fields
	fields := `["name"]`
	if listFields != "" {
		fields = listFields
	}

	// Escape filters
	filters := "None"
	if listFilters != "" {
		escaped := strings.ReplaceAll(listFilters, `'`, `\'`)
		filters = fmt.Sprintf("json.loads('%s')", escaped)
	}

	orderBy := ""
	if listOrderBy != "" {
		orderBy = listOrderBy
	}

	executor := api.NewExecutor(benchPath, site, "Administrator")

	script := fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()

try:
    filters = %s
    fields = json.loads('%s')
    order_by = '%s' if '%s' else None

    results = frappe.get_list('%s',
        filters=filters,
        fields=fields,
        limit_page_length=%d,
        order_by=order_by
    )
    print(json.dumps({"success": True, "data": results}, default=str))
except Exception as ex:
    import traceback
    print(json.dumps({"success": False, "error": str(ex), "traceback": traceback.format_exc()}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site, filters, fields, orderBy, orderBy, doctype, listLimit)

	result, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to list documents: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("failed to list documents: %s", result.Error)
	}

	// Format output as table
	docs, ok := result.Data.([]any)
	if !ok || len(docs) == 0 {
		wegoutput.Print("No documents found")
		return nil
	}

	output, _ := json.MarshalIndent(docs, "", "  ")
	wegoutput.Print(string(output))
	return nil
}
