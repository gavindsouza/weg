package scheduler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/api"
	"github.com/gavindsouza/weg/internal/config"
	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var statusSite string

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check scheduler status",
	Long: `Check if the scheduler is enabled or disabled.

Examples:
  weg scheduler status
  weg scheduler status --site mysite.localhost`,
	RunE: runStatus,
}

func init() {
	statusCmd.Flags().StringVar(&statusSite, "site", "", "Site to check")
}

func runStatus(cmd *cobra.Command, args []string) error {
	benchPath, site, err := resolveSite(statusSite)
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
    enabled = not frappe.utils.scheduler.is_scheduler_disabled()
    print(json.dumps({"success": True, "data": {"enabled": enabled}}))
except Exception as ex:
    print(json.dumps({"success": False, "error": str(ex)}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site)

	result, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to check scheduler status: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("failed to check scheduler status: %s", result.Error)
	}

	data, ok := result.Data.(map[string]any)
	if !ok {
		return fmt.Errorf("unexpected response format")
	}

	enabled, _ := data["enabled"].(bool)
	if enabled {
		fmt.Printf("Scheduler is enabled for %s\n", site)
	} else {
		fmt.Printf("Scheduler is disabled for %s\n", site)
	}

	return nil
}

// resolveSite resolves the bench path and site name
func resolveSite(siteName string) (string, string, error) {
	path := "."
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", "", fmt.Errorf("invalid path: %w", err)
	}

	result, err := config.DetectProjectContext(absPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to detect context: %w", err)
	}

	var benchPath string
	switch result.Context {
	case config.ContextWegBench:
		benchPath = result.BenchPath
	case config.ContextWegApp:
		benchPath = result.BenchPath
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
