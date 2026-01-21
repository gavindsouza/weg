package doc

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/api"
	"github.com/gavindsouza/weg/internal/completion"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/prompt"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <doctype> <name>",
	Short: "Delete a document",
	Long: `Delete a document by doctype and name.

Examples:
  weg doc delete User test@test.com
  weg doc delete "Sales Invoice" INV-001
  weg doc delete Item OLD-ITEM --force`,
	Args:              cobra.ExactArgs(2),
	RunE:              runDelete,
	ValidArgsFunction: completion.CompleteDocTypesForArg(0),
}

var (
	deleteSite  string
	deleteForce bool
)

func init() {
	DocCmd.AddCommand(deleteCmd)
	deleteCmd.Flags().StringVarP(&deleteSite, "site", "s", "", "Site to delete from")
	deleteCmd.Flags().BoolVar(&deleteForce, "force", false, "Skip confirmation")
}

func runDelete(cmd *cobra.Command, args []string) error {
	doctype := args[0]
	name := args[1]

	benchPath, site, err := resolveContext(deleteSite)
	if err != nil {
		return err
	}

	if !deleteForce {
		output.Printf("Delete %s/%s from %s?", doctype, name, site)
		if !prompt.Confirm("Continue?") {
			output.Print("Cancelled")
			return nil
		}
	}

	executor := api.NewExecutor(benchPath, site, "Administrator")

	script := fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()

try:
    frappe.delete_doc('%s', '%s', force=True)
    frappe.db.commit()
    print(json.dumps({"success": True}))
except Exception as ex:
    frappe.db.rollback()
    import traceback
    print(json.dumps({"success": False, "error": str(ex), "traceback": traceback.format_exc()}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site, doctype, name)

	result, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}

	if !result.Success {
		if result.Traceback != "" {
			fmt.Fprintf(os.Stderr, "%s\n", result.Traceback)
		}
		return fmt.Errorf("failed to delete document: %s", result.Error)
	}

	output.Successf("Deleted %s/%s", doctype, name)
	return nil
}
