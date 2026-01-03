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

var trimTablesCmd = &cobra.Command{
	Use:   "trim-tables",
	Short: "Trim log tables to reduce database size",
	Long: `Trim log tables that grow large over time.

This cleans up tables like Error Log, Activity Log, Email Queue,
and other log tables that can consume significant database space.

Examples:
  weg trim-tables
  weg trim-tables --days 30       # Keep last 30 days (default)
  weg trim-tables --days 7        # Keep only last week
  weg trim-tables --site test`,
	RunE: runTrimTables,
}

var (
	trimTablesSite string
	trimTablesDays int
)

func init() {
	rootCmd.AddCommand(trimTablesCmd)
	trimTablesCmd.Flags().StringVar(&trimTablesSite, "site", "", "Site to trim tables for")
	trimTablesCmd.Flags().IntVar(&trimTablesDays, "days", 30, "Keep records from last N days")
}

func runTrimTables(cmd *cobra.Command, args []string) error {
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
	site := trimTablesSite
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

	PrintInfo("Trimming log tables for %s (keeping last %d days)...", site, trimTablesDays)

	executor := api.NewExecutor(benchPath, site, "Administrator")

	script := fmt.Sprintf(`import frappe
import json
import os
from datetime import datetime, timedelta

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()

try:
    cutoff = datetime.now() - timedelta(days=%d)
    cutoff_str = cutoff.strftime('%%Y-%%m-%%d')

    tables_to_trim = [
        ('Error Log', 'creation'),
        ('Activity Log', 'creation'),
        ('Email Queue', 'creation'),
        ('Scheduled Job Log', 'creation'),
        ('Version', 'creation'),
        ('View Log', 'creation'),
        ('Access Log', 'creation'),
        ('Route History', 'creation'),
    ]

    results = {}
    for doctype, date_field in tables_to_trim:
        try:
            if frappe.db.table_exists('tab' + doctype.replace(' ', '')):
                count = frappe.db.count(doctype, {date_field: ['<', cutoff_str]})
                if count > 0:
                    frappe.db.delete(doctype, {date_field: ['<', cutoff_str]})
                    results[doctype] = count
        except Exception:
            pass  # Table might not exist or have different structure

    frappe.db.commit()
    print(json.dumps({"success": True, "data": results}))
except Exception as ex:
    frappe.db.rollback()
    import traceback
    print(json.dumps({"success": False, "error": str(ex), "traceback": traceback.format_exc()}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site, trimTablesDays)

	apiResult, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to trim tables: %w", err)
	}

	if !apiResult.Success {
		if apiResult.Traceback != "" {
			fmt.Fprintf(os.Stderr, "%s\n", apiResult.Traceback)
		}
		return fmt.Errorf("failed to trim tables: %s", apiResult.Error)
	}

	results, ok := apiResult.Data.(map[string]interface{})
	if ok && len(results) > 0 {
		PrintInfo("Deleted records:")
		for table, count := range results {
			PrintInfo("  %s: %.0f", table, count)
		}
	} else {
		PrintInfo("No old records found to delete")
	}

	return nil
}
