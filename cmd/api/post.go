package api

import (
	"encoding/json"
	"fmt"

	internalapi "github.com/gavindsouza/weg/internal/api"
	"github.com/gavindsouza/weg/internal/remote"
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
	doctype := args[0]

	// Parse document data
	var doc map[string]any
	if err := json.Unmarshal([]byte(postData), &doc); err != nil {
		return fmt.Errorf("invalid JSON data: %w", err)
	}

	// Remote mode
	if isRemoteMode() {
		key, secret, err := resolveRemoteCredentials()
		if err != nil {
			return err
		}

		client := remote.NewClient(apiURL, key, secret)
		result, err := remoteCreate(client, doctype, doc)
		if err != nil {
			return err
		}
		return printRemoteResult(result)
	}

	// Local mode - add doctype to document
	doc["doctype"] = doctype

	benchPath, site, err := detectBenchAndSite()
	if err != nil {
		return err
	}

	executor := internalapi.NewExecutor(benchPath, site, apiUser)
	result, err := executor.Insert(doc)
	if err != nil {
		return err
	}

	return printResult(result)
}
