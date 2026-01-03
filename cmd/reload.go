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

var reloadDocCmd = &cobra.Command{
	Use:   "reload-doc <app> <module> <doctype>",
	Short: "Reload a DocType from disk",
	Long: `Reload a DocType definition from its JSON file on disk.

This is useful during development when you've modified a DocType's
JSON definition and want to apply the changes without a full migrate.

Examples:
  weg reload-doc myapp mymodule MyDocType
  weg reload-doc frappe core User
  weg reload-doc myapp api ContentRelationship --site test`,
	Args: cobra.ExactArgs(3),
	RunE: runReloadDoc,
}

var reloadDocSite string

func init() {
	rootCmd.AddCommand(reloadDocCmd)
	reloadDocCmd.Flags().StringVar(&reloadDocSite, "site", "", "Site to reload on")
}

func runReloadDoc(cmd *cobra.Command, args []string) error {
	appName := args[0]
	moduleName := args[1]
	doctype := args[2]

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
	site := reloadDocSite
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

	PrintInfo("Reloading %s.%s.%s on %s...", appName, moduleName, doctype, site)

	executor := api.NewExecutor(benchPath, site, "Administrator")

	script := fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()

try:
    frappe.reload_doc('%s', '%s', '%s')
    frappe.db.commit()
    print(json.dumps({"success": True, "data": "reloaded"}))
except Exception as ex:
    frappe.db.rollback()
    import traceback
    print(json.dumps({"success": False, "error": str(ex), "traceback": traceback.format_exc()}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site, appName, moduleName, strings.ToLower(strings.ReplaceAll(doctype, " ", "_")))

	apiResult, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to reload doc: %w", err)
	}

	if !apiResult.Success {
		if apiResult.Traceback != "" {
			fmt.Fprintf(os.Stderr, "%s\n", apiResult.Traceback)
		}
		return fmt.Errorf("failed to reload doc: %s", apiResult.Error)
	}

	PrintInfo("DocType %s reloaded successfully", doctype)
	return nil
}
