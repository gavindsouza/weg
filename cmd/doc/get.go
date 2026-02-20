package doc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/api"
	"github.com/gavindsouza/weg/internal/completion"
	"github.com/gavindsouza/weg/internal/config"
	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:   "get <doctype> <name>",
	Short: "Get a document",
	Long: `Retrieve a document by doctype and name.

Examples:
  weg doc get User Administrator
  weg doc get "Sales Invoice" INV-001
  weg doc get User test@test.com --json`,
	Args:              cobra.ExactArgs(2),
	RunE:              runGet,
	ValidArgsFunction: completion.CompleteDocTypesForArg(0),
}

var (
	getSite string
	getJSON bool
)

func init() {
	DocCmd.AddCommand(getCmd)
	getCmd.Flags().StringVarP(&getSite, "site", "s", "", "Site to query")
	getCmd.Flags().BoolVar(&getJSON, "json", false, "Output raw JSON")
}

func runGet(cmd *cobra.Command, args []string) error {
	doctype := args[0]
	name := args[1]

	benchPath, site, err := resolveContext(getSite)
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
    print(json.dumps({"success": True, "data": doc.as_dict()}, default=str))
except Exception as ex:
    import traceback
    print(json.dumps({"success": False, "error": str(ex), "traceback": traceback.format_exc()}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site, doctype, name)

	result, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to get document: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("failed to get document: %s", result.Error)
	}

	output, _ := json.MarshalIndent(result.Data, "", "  ")
	fmt.Println(string(output))
	return nil
}

// resolveContext is a helper to get benchPath and site
func resolveContext(siteName string) (string, string, error) {
	path := "."
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", "", fmt.Errorf("invalid path: %w", err)
	}

	result, err := config.DetectContext(absPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to detect context: %w", err)
	}

	var benchPath string
	switch result.Context {
	case config.ContextWegBench:
		benchPath = absPath
	case config.ContextWegApp:
		benchPath = filepath.Join(absPath, ".weg")
	default:
		return "", "", wegerrors.NotInProject(absPath)
	}

	site := siteName
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
		return "", "", fmt.Errorf("no site specified and no default site found")
	}

	return benchPath, site, nil
}
