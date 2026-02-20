package db

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/config"
	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var queryCmd = &cobra.Command{
	Use:   "query <sql>",
	Short: "Run SQL query against database",
	Long: `Execute SQL queries directly against the site database.

Uses frappe.db.sql under the hood, so results are returned as JSON.
Use --raw for unformatted output suitable for piping.

Examples:
  weg db query "SELECT name, email FROM tabUser LIMIT 5"
  weg db query "SELECT COUNT(*) FROM tabToDo WHERE status='Open'"
  weg db query --site dev.localhost "SHOW TABLES"
  echo "SELECT 1" | weg db query -   # Read from stdin`,
	Args: cobra.ExactArgs(1),
	RunE: runQuery,
}

var (
	querySite   string
	queryRaw    bool
	queryAsDict bool
)

func init() {
	DbCmd.AddCommand(queryCmd)
	queryCmd.Flags().StringVarP(&querySite, "site", "s", "", "Target site (default: auto-detect)")
	queryCmd.Flags().BoolVar(&queryRaw, "raw", false, "Output raw JSON without formatting")
	queryCmd.Flags().BoolVar(&queryAsDict, "as-dict", true, "Return results as list of dicts (default: true)")
}

func runQuery(cmd *cobra.Command, args []string) error {
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
		return wegerrors.NotInProject(absPath)
	}

	// Determine site
	site := querySite
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

	// Get SQL - either from arg or stdin
	sql := args[0]
	if sql == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
		sql = strings.TrimSpace(string(data))
	}

	if sql == "" {
		return fmt.Errorf("empty SQL query")
	}

	// Build Python code to execute
	asDictStr := "True"
	if !queryAsDict {
		asDictStr = "False"
	}

	indentArg := "2"
	if queryRaw {
		indentArg = "None"
	}

	pythonCode := fmt.Sprintf(`
import json
import frappe
frappe.connect(site=%q)
try:
    result = frappe.db.sql(%q, as_dict=%s)
    print(json.dumps(result, default=str, indent=%s))
finally:
    frappe.destroy()
`, site, sql, asDictStr, indentArg)

	// Build PYTHONPATH
	appsDir := filepath.Join(benchPath, "apps")
	entries, _ := os.ReadDir(appsDir)
	var pythonPaths []string
	for _, entry := range entries {
		if entry.IsDir() {
			pythonPaths = append(pythonPaths, filepath.Join(appsDir, entry.Name()))
		}
	}
	pythonPathEnv := strings.Join(pythonPaths, ":")

	// Execute via Python
	pyCmd := exec.Command(filepath.Join(benchPath, "env", "bin", "python"), "-c", pythonCode)
	pyCmd.Dir = filepath.Join(benchPath, "sites")
	pyCmd.Env = append(os.Environ(), "PYTHONPATH="+pythonPathEnv)
	pyCmd.Stderr = os.Stderr

	output, err := pyCmd.Output()
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	// Format output
	if queryRaw {
		fmt.Print(string(output))
	} else {
		// Pretty print JSON
		var data interface{}
		if err := json.Unmarshal(output, &data); err != nil {
			// Not JSON, print as-is
			fmt.Print(string(output))
		} else {
			formatted, _ := json.MarshalIndent(data, "", "  ")
			fmt.Println(string(formatted))
		}
	}

	return nil
}
