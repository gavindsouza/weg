package api

import (
	"encoding/json"
	"fmt"

	internalapi "github.com/gavindsouza/weg/internal/api"
	"github.com/spf13/cobra"
)

var postData string

var postCmd = &cobra.Command{
	Use:   "post <doctype>",
	Short: "Create a new document",
	Long: `Create a new document in Frappe.

The document data must be provided as JSON via the --data flag.
The doctype will be automatically added to the document.

Examples:
  weg api post User -d '{"email":"test@example.com","first_name":"Test"}'
  weg api post "Sales Invoice" -d '{"customer":"CUST-001","items":[...]}'`,
	Args:         cobra.ExactArgs(1),
	RunE:         runPost,
	SilenceUsage: true,
}

func init() {
	postCmd.Flags().StringVarP(&postData, "data", "d", "", "JSON document data (required)")
	postCmd.MarkFlagRequired("data")
}

func runPost(cmd *cobra.Command, args []string) error {
	benchPath, site, err := detectBenchAndSite()
	if err != nil {
		return err
	}

	doctype := args[0]

	// Parse document data
	var doc map[string]interface{}
	if err := json.Unmarshal([]byte(postData), &doc); err != nil {
		return fmt.Errorf("invalid JSON data: %w", err)
	}

	// Add doctype to document
	doc["doctype"] = doctype

	executor := internalapi.NewExecutor(benchPath, site, apiUser)
	result, err := executor.Insert(doc)
	if err != nil {
		return err
	}

	return printResult(result)
}
