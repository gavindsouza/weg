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

var importDocCmd = &cobra.Command{
	Use:   "import-doc <file.json>",
	Short: "Import a document from a JSON file",
	Long: `Import a document from a JSON file into the database.

The JSON file should contain a valid Frappe document with a doctype field.
If the document already exists, it will be updated (unless --no-update is set).

Examples:
  weg import-doc user.json
  weg import-doc fixtures/role.json --site test
  weg import-doc data.json --no-update  # Skip if exists`,
	Args: cobra.ExactArgs(1),
	RunE: runImportDoc,
}

var (
	importDocSite     string
	importDocNoUpdate bool
)

func init() {
	rootCmd.AddCommand(importDocCmd)
	importDocCmd.Flags().StringVar(&importDocSite, "site", "", "Site to import into")
	importDocCmd.Flags().BoolVar(&importDocNoUpdate, "no-update", false, "Skip if document exists")
}

func runImportDoc(cmd *cobra.Command, args []string) error {
	jsonFile := args[0]

	// Read the JSON file
	data, err := os.ReadFile(jsonFile)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Parse to validate and get doctype
	var doc map[string]interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	doctype, ok := doc["doctype"].(string)
	if !ok || doctype == "" {
		return fmt.Errorf("JSON must contain a 'doctype' field")
	}

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
	site := importDocSite
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

	// Escape JSON for Python
	escapedJSON := strings.ReplaceAll(string(data), `\`, `\\`)
	escapedJSON = strings.ReplaceAll(escapedJSON, `'`, `\'`)
	escapedJSON = strings.ReplaceAll(escapedJSON, "\n", "\\n")
	escapedJSON = strings.ReplaceAll(escapedJSON, "\r", "")

	updateMode := "true"
	if importDocNoUpdate {
		updateMode = "false"
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

	apiResult, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to import doc: %w", err)
	}

	if !apiResult.Success {
		if apiResult.Traceback != "" {
			fmt.Fprintf(os.Stderr, "%s\n", apiResult.Traceback)
		}
		return fmt.Errorf("failed to import doc: %s", apiResult.Error)
	}

	resultData, ok := apiResult.Data.(map[string]interface{})
	if ok {
		action := resultData["action"]
		name := resultData["name"]
		switch action {
		case "inserted":
			PrintInfo("Inserted %s: %s", doctype, name)
		case "updated":
			PrintInfo("Updated %s: %s", doctype, name)
		case "skipped":
			PrintInfo("Skipped %s: %s (already exists)", doctype, name)
		}
	}

	return nil
}
