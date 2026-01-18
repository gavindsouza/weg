package doc

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/api"
	"github.com/spf13/cobra"
)

var renameCmd = &cobra.Command{
	Use:   "rename <doctype> <old-name> <new-name>",
	Short: "Rename a document",
	Long: `Rename a document and update all references.

Examples:
  weg doc rename Customer "Old Name" "New Name"
  weg doc rename Item ITEM-001 ITEM-NEW-001
  weg doc rename User old@email.com new@email.com --merge`,
	Args: cobra.ExactArgs(3),
	RunE: runRename,
}

var (
	renameSite  string
	renameMerge bool
)

func init() {
	DocCmd.AddCommand(renameCmd)
	renameCmd.Flags().StringVarP(&renameSite, "site", "s", "", "Site to rename in")
	renameCmd.Flags().BoolVar(&renameMerge, "merge", false, "Merge if target exists")
}

func runRename(cmd *cobra.Command, args []string) error {
	doctype := args[0]
	oldName := args[1]
	newName := args[2]

	benchPath, site, err := resolveContext(renameSite)
	if err != nil {
		return err
	}

	merge := "False"
	if renameMerge {
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

	result, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to rename: %w", err)
	}

	if !result.Success {
		if result.Traceback != "" {
			fmt.Fprintf(os.Stderr, "%s\n", result.Traceback)
		}
		return fmt.Errorf("failed to rename: %s", result.Error)
	}

	fmt.Printf("Renamed %s: %s -> %s\n", doctype, oldName, result.Data)
	return nil
}
