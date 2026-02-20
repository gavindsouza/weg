package doc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/api"
	"github.com/spf13/cobra"
)

var importCmd = &cobra.Command{
	Use:   "import <file.json>",
	Short: "Import a document from JSON",
	Long: `Import a document from a JSON file.

If the document exists, it will be updated (unless --no-update).

Examples:
  weg doc import user.json
  weg doc import data.json --no-update
  weg doc import fixtures/role.json --site test`,
	Args: cobra.ExactArgs(1),
	RunE: runImport,
}

var (
	importSite     string
	importNoUpdate bool
)

func init() {
	DocCmd.AddCommand(importCmd)
	importCmd.Flags().StringVarP(&importSite, "site", "s", "", "Site to import into")
	importCmd.Flags().BoolVar(&importNoUpdate, "no-update", false, "Skip if document exists")
}

func runImport(cmd *cobra.Command, args []string) error {
	jsonFile := args[0]

	data, err := os.ReadFile(jsonFile)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	doctype, ok := doc["doctype"].(string)
	if !ok || doctype == "" {
		return fmt.Errorf("JSON must contain a 'doctype' field")
	}

	benchPath, site, err := resolveContext(importSite)
	if err != nil {
		return err
	}

	escapedJSON := strings.ReplaceAll(string(data), `\`, `\\`)
	escapedJSON = strings.ReplaceAll(escapedJSON, `'`, `\'`)
	escapedJSON = strings.ReplaceAll(escapedJSON, "\n", "\\n")
	escapedJSON = strings.ReplaceAll(escapedJSON, "\r", "")

	updateMode := "True"
	if importNoUpdate {
		updateMode = "False"
	}

	executor := api.NewExecutor(benchPath, site, "Administrator")

	script := fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()

try:
    doc_data = json.loads('%s')
    doctype = doc_data.get('doctype')
    name = doc_data.get('name')
    update_if_exists = %s

    exists = False
    if name:
        exists = frappe.db.exists(doctype, name)

    if exists and not update_if_exists:
        print(json.dumps({"success": True, "data": {"action": "skipped", "name": name}}))
    elif exists:
        doc = frappe.get_doc(doctype, name)
        doc.update(doc_data)
        doc.save()
        frappe.db.commit()
        print(json.dumps({"success": True, "data": {"action": "updated", "name": doc.name}}))
    else:
        doc = frappe.get_doc(doc_data)
        doc.insert()
        frappe.db.commit()
        print(json.dumps({"success": True, "data": {"action": "inserted", "name": doc.name}}))
except Exception as ex:
    frappe.db.rollback()
    import traceback
    print(json.dumps({"success": False, "error": str(ex), "traceback": traceback.format_exc()}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site, escapedJSON, updateMode)

	result, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to import: %w", err)
	}

	if !result.Success {
		if result.Traceback != "" {
			fmt.Fprintf(os.Stderr, "%s\n", result.Traceback)
		}
		return fmt.Errorf("failed to import: %s", result.Error)
	}

	resultData, ok := result.Data.(map[string]any)
	if ok {
		action := resultData["action"]
		name := resultData["name"]
		switch action {
		case "inserted":
			fmt.Printf("Inserted %s: %s\n", doctype, name)
		case "updated":
			fmt.Printf("Updated %s: %s\n", doctype, name)
		case "skipped":
			fmt.Printf("Skipped %s: %s (already exists)\n", doctype, name)
		}
	}

	return nil
}
