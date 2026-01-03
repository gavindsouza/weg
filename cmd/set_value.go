package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gavindsouza/weg/internal/api"
	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var setValueCmd = &cobra.Command{
	Use:   "set-value <doctype> <name> <field> <value>",
	Short: "Set a field value on a document",
	Long: `Set a field value on a document directly.

This updates a single field without loading the full document,
making it fast for simple updates.

Examples:
  weg set-value User Administrator enabled 0
  weg set-value "Sales Invoice" INV-001 status Paid
  weg set-value User test@test.com user_type "Website User"`,
	Args: cobra.ExactArgs(4),
	RunE: runSetValue,
}

var setValueSite string

func init() {
	rootCmd.AddCommand(setValueCmd)
	setValueCmd.Flags().StringVar(&setValueSite, "site", "", "Site to update")
}

func runSetValue(cmd *cobra.Command, args []string) error {
	doctype := args[0]
	name := args[1]
	field := args[2]
	valueStr := args[3]

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

	site := setValueSite
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

	// Parse value
	var valuePython string
	if valueStr == "true" || valueStr == "True" {
		valuePython = "True"
	} else if valueStr == "false" || valueStr == "False" {
		valuePython = "False"
	} else if valueStr == "null" || valueStr == "None" {
		valuePython = "None"
	} else if i, err := strconv.Atoi(valueStr); err == nil {
		valuePython = fmt.Sprintf("%d", i)
	} else if f, err := strconv.ParseFloat(valueStr, 64); err == nil {
		valuePython = fmt.Sprintf("%f", f)
	} else {
		valuePython = fmt.Sprintf("'%s'", strings.ReplaceAll(valueStr, "'", "\\'"))
	}

	executor := api.NewExecutor(benchPath, site, "Administrator")

	script := fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()

try:
    frappe.db.set_value('%s', '%s', '%s', %s)
    frappe.db.commit()
    print(json.dumps({"success": True}))
except Exception as ex:
    frappe.db.rollback()
    import traceback
    print(json.dumps({"success": False, "error": str(ex), "traceback": traceback.format_exc()}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site, doctype, name, field, valuePython)

	apiResult, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to set value: %w", err)
	}

	if !apiResult.Success {
		if apiResult.Traceback != "" {
			fmt.Fprintf(os.Stderr, "%s\n", apiResult.Traceback)
		}
		return fmt.Errorf("failed to set value: %s", apiResult.Error)
	}

	PrintInfo("Set %s.%s = %s on %s/%s", doctype, field, valueStr, doctype, name)
	return nil
}
