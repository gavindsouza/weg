package api

import (
	"encoding/json"
	"fmt"
	"strings"

	internalapi "github.com/gavindsouza/weg/internal/api"
	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/remote"
	"github.com/spf13/cobra"
)

var putData string

var putCmd = &cobra.Command{
	Use:   "put <doctype>/<name>",
	Short: "Update an existing document",
	Long: `Update an existing document in Frappe.

The document data must be provided as JSON via the --data flag.
The doctype and name will be automatically added to the document.

Examples:
  weg api put User/test@example.com -d '{"first_name":"Updated"}'
  weg api put "Sales Invoice/INV-001" -d '{"status":"Paid"}'`,
	Args:         cobra.ExactArgs(1),
	RunE:         runPut,
	SilenceUsage: true,
}

func init() {
	putCmd.Flags().StringVarP(&putData, "data", "d", "", "JSON document data (required)")
	putCmd.MarkFlagRequired("data")
}

func runPut(cmd *cobra.Command, args []string) error {
	// Parse doctype/name
	arg := args[0]
	if !strings.Contains(arg, "/") {
		return wegerrors.Usage("expected format: <doctype>/<name>")
	}

	parts := strings.SplitN(arg, "/", 2)
	doctype := parts[0]
	name := parts[1]

	// Parse document data
	var doc map[string]any
	if err := json.Unmarshal([]byte(putData), &doc); err != nil {
		return wegerrors.Validation("data", fmt.Sprintf("invalid JSON: %v", err))
	}

	// Remote mode
	if isRemoteMode() {
		key, secret, err := resolveRemoteCredentials()
		if err != nil {
			return err
		}

		client := remote.NewClient(apiURL, key, secret)
		result, err := remoteUpdate(client, doctype, name, doc)
		if err != nil {
			return err
		}
		return printRemoteResult(result)
	}

	// Local mode - add doctype and name to document
	doc["doctype"] = doctype
	doc["name"] = name

	benchPath, site, err := detectBenchAndSite()
	if err != nil {
		return err
	}

	executor := internalapi.NewExecutor(benchPath, site, apiUser)
	result, err := executor.Save(doc)
	if err != nil {
		return err
	}

	return printResult(result)
}
