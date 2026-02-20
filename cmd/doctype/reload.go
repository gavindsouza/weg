package doctype

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/api"
	"github.com/gavindsouza/weg/internal/completion"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/spf13/cobra"
)

var reloadCmd = &cobra.Command{
	Use:   "reload <app> <module> <doctype>",
	Short: "Reload DocType from disk",
	Long: `Reload a DocType definition from its JSON file.

This is useful when you've modified a DocType's JSON definition
and want to apply changes without a full migration.

Examples:
  weg doctype reload myapp mymodule MyDocType
  weg doctype reload frappe core User
  weg doctype reload erpnext stock Item`,
	Args:              cobra.ExactArgs(3),
	RunE:              runReload,
	ValidArgsFunction: completion.CompleteAppNamesForArg(0),
}

var reloadSite string

func init() {
	DoctypeCmd.AddCommand(reloadCmd)
	reloadCmd.Flags().StringVarP(&reloadSite, "site", "s", "", "Site to reload on")
}

func runReload(cmd *cobra.Command, args []string) error {
	appName := args[0]
	moduleName := args[1]
	doctype := args[2]

	benchPath, site, err := resolveContext(reloadSite)
	if err != nil {
		return err
	}

	output.Infof("Reloading %s.%s.%s on %s...\n", appName, moduleName, doctype, site)

	executor := api.NewExecutor(benchPath, site, "Administrator")

	// Convert doctype name to filename format (lowercase, underscores)
	doctypeFile := strings.ToLower(strings.ReplaceAll(doctype, " ", "_"))

	script := fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()

try:
    frappe.reload_doc('%s', '%s', '%s')
    frappe.db.commit()
    print(json.dumps({"success": True}))
except Exception as ex:
    frappe.db.rollback()
    import traceback
    print(json.dumps({"success": False, "error": str(ex), "traceback": traceback.format_exc()}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site, appName, moduleName, doctypeFile)

	result, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to reload: %w", err)
	}

	if !result.Success {
		if result.Traceback != "" {
			output.Errorf("%s", result.Traceback)
		}
		return fmt.Errorf("failed to reload: %s", result.Error)
	}

	output.Printf("DocType %s reloaded successfully", doctype)
	return nil
}
