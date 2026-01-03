package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/api"
	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var renameDocCmd = &cobra.Command{
	Use:   "rename-doc <doctype> <old-name> <new-name>",
	Short: "Rename a document",
	Long: `Rename a document and update all references.

This performs a proper rename operation that updates
linked documents and references throughout the system.

Examples:
  weg rename-doc Customer "Old Name" "New Name"
  weg rename-doc Item ITEM-001 ITEM-NEW-001
  weg rename-doc "Sales Partner" old-partner new-partner`,
	Args: cobra.ExactArgs(3),
	RunE: runRenameDoc,
}

var (
	renameDocSite  string
	renameDocMerge bool
)

func init() {
	rootCmd.AddCommand(renameDocCmd)
	renameDocCmd.Flags().StringVar(&renameDocSite, "site", "", "Site to rename in")
	renameDocCmd.Flags().BoolVar(&renameDocMerge, "merge", false, "Merge into existing document if it exists")
}

func runRenameDoc(cmd *cobra.Command, args []string) error {
	doctype := args[0]
	oldName := args[1]
	newName := args[2]

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

	site := renameDocSite
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

	merge := "False"
	if renameDocMerge {
		merge = "True"
	}

	executor := api.NewExecutor(benchPath, site, "Administrator")

	script := fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()

try:
    from frappe.model.rename_doc import rename_doc
    new_name = rename_doc('%s', '%s', '%s', merge=%s)
    frappe.db.commit()
    print(json.dumps({"success": True, "data": new_name}))
except Exception as ex:
    frappe.db.rollback()
    import traceback
    print(json.dumps({"success": False, "error": str(ex), "traceback": traceback.format_exc()}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site, doctype, oldName, newName, merge)

	apiResult, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to rename doc: %w", err)
	}

	if !apiResult.Success {
		if apiResult.Traceback != "" {
			fmt.Fprintf(os.Stderr, "%s\n", apiResult.Traceback)
		}
		return fmt.Errorf("failed to rename doc: %s", apiResult.Error)
	}

	PrintInfo("Renamed %s: %s -> %s", doctype, oldName, apiResult.Data)
	return nil
}
