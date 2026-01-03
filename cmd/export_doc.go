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

var exportDocCmd = &cobra.Command{
	Use:   "export-doc <doctype> <name>",
	Short: "Export a document to JSON",
	Long: `Export a document from the database to a JSON file.

By default prints to stdout. Use --output to write to a file.

Examples:
  weg export-doc User Administrator
  weg export-doc "Sales Invoice" INV-001 --output invoice.json
  weg export-doc Role "System Manager" --site test`,
	Args: cobra.ExactArgs(2),
	RunE: runExportDoc,
}

var (
	exportDocSite   string
	exportDocOutput string
)

func init() {
	rootCmd.AddCommand(exportDocCmd)
	exportDocCmd.Flags().StringVar(&exportDocSite, "site", "", "Site to export from")
	exportDocCmd.Flags().StringVarP(&exportDocOutput, "output", "o", "", "Output file (default: stdout)")
}

func runExportDoc(cmd *cobra.Command, args []string) error {
	doctype := args[0]
	name := args[1]

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
	site := exportDocSite
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
    doc = frappe.get_doc('%s', '%s')
    data = doc.as_dict()
    # Remove internal fields for cleaner export
    for key in ['modified', 'modified_by', 'creation', 'owner', 'idx', 'docstatus']:
        data.pop(key, None)
    print(json.dumps({"success": True, "data": data}, default=str, indent=2))
except Exception as ex:
    import traceback
    print(json.dumps({"success": False, "error": str(ex), "traceback": traceback.format_exc()}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site, doctype, name)

	apiResult, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to export doc: %w", err)
	}

	if !apiResult.Success {
		if apiResult.Traceback != "" {
			fmt.Fprintf(os.Stderr, "%s\n", apiResult.Traceback)
		}
		return fmt.Errorf("failed to export doc: %s", apiResult.Error)
	}

	// Format the output
	output, err := json.MarshalIndent(apiResult.Data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	if exportDocOutput != "" {
		if err := os.WriteFile(exportDocOutput, output, 0644); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}
		PrintInfo("Exported %s/%s to %s", doctype, name, exportDocOutput)
	} else {
		fmt.Println(string(output))
	}

	return nil
}
