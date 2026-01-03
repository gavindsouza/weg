package doc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/api"
	"github.com/spf13/cobra"
)

var exportCmd = &cobra.Command{
	Use:   "export <doctype> <name>",
	Short: "Export a document to JSON",
	Long: `Export a document to a JSON file or stdout.

Examples:
  weg doc export User Administrator
  weg doc export "Sales Invoice" INV-001 -o invoice.json
  weg doc export Role "System Manager" --site test`,
	Args: cobra.ExactArgs(2),
	RunE: runExport,
}

var (
	exportSite   string
	exportOutput string
)

func init() {
	DocCmd.AddCommand(exportCmd)
	exportCmd.Flags().StringVar(&exportSite, "site", "", "Site to export from")
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "Output file (default: stdout)")
}

func runExport(cmd *cobra.Command, args []string) error {
	doctype := args[0]
	name := args[1]

	benchPath, site, err := resolveContext(exportSite)
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
    doc = frappe.get_doc('%s', '%s')
    data = doc.as_dict()
    # Remove internal fields for cleaner export
    for key in ['modified', 'modified_by', 'creation', 'owner', 'idx', 'docstatus']:
        data.pop(key, None)
    print(json.dumps({"success": True, "data": data}, default=str))
except Exception as ex:
    import traceback
    print(json.dumps({"success": False, "error": str(ex), "traceback": traceback.format_exc()}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site, doctype, name)

	result, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to export: %w", err)
	}

	if !result.Success {
		if result.Traceback != "" {
			fmt.Fprintf(os.Stderr, "%s\n", result.Traceback)
		}
		return fmt.Errorf("failed to export: %s", result.Error)
	}

	output, _ := json.MarshalIndent(result.Data, "", "  ")

	if exportOutput != "" {
		if err := os.WriteFile(exportOutput, output, 0644); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}
		fmt.Printf("Exported %s/%s to %s\n", doctype, name, exportOutput)
	} else {
		fmt.Println(string(output))
	}

	return nil
}
