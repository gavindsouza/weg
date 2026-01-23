package api

import (
	"encoding/json"
	"fmt"
	"strings"

	internalapi "github.com/gavindsouza/weg/internal/api"
	"github.com/gavindsouza/weg/internal/remote"
	"github.com/spf13/cobra"
)

var (
	getFilters string
	getFields  string
	getLimit   int
	getOrderBy string
)

var getCmd = &cobra.Command{
	Use:   "get <doctype>[/<name>]",
	Short: "Get document(s) from Frappe",
	Long: `Retrieve one or more documents from Frappe.

If only doctype is provided, returns a list of documents.
If doctype/name is provided, returns a single document.

Examples:
  weg api get User                              # List all Users
  weg api get User/Administrator                # Get specific User
  weg api get User -F '{"enabled":1}'            # Filter results
  weg api get User --fields '["name","email"]'  # Select fields
  weg api get User --limit 10                   # Limit results
  weg api get "Sales Invoice" --order-by "creation desc"`,
	Args:         cobra.ExactArgs(1),
	RunE:         runGet,
	SilenceUsage: true,
}

func init() {
	getCmd.Flags().StringVarP(&getFilters, "filters", "F", "", "JSON filter object")
	getCmd.Flags().StringVar(&getFields, "fields", "", "JSON array of fields to return")
	getCmd.Flags().IntVarP(&getLimit, "limit", "l", 20, "Maximum number of results")
	getCmd.Flags().StringVar(&getOrderBy, "order-by", "", "Order by field (e.g. 'creation desc')")
}

func runGet(cmd *cobra.Command, args []string) error {
	// Parse doctype and optional name
	arg := args[0]
	var doctype, name string

	if strings.Contains(arg, "/") {
		parts := strings.SplitN(arg, "/", 2)
		doctype = parts[0]
		name = parts[1]
	} else {
		doctype = arg
	}

	// Parse filters and fields
	var filters map[string]any
	var fields []string

	if getFilters != "" {
		if err := json.Unmarshal([]byte(getFilters), &filters); err != nil {
			return fmt.Errorf("invalid filters JSON: %w", err)
		}
	}

	if getFields != "" {
		if err := json.Unmarshal([]byte(getFields), &fields); err != nil {
			return fmt.Errorf("invalid fields JSON: %w", err)
		}
	}

	// Remote mode
	if isRemoteMode() {
		key, secret, credErr := resolveRemoteCredentials()
		if credErr != nil {
			return credErr
		}

		client := remote.NewClient(apiURL, key, secret)
		var result *RemoteResult
		var remoteErr error

		if name != "" {
			result, remoteErr = remoteGetDoc(client, doctype, name)
		} else {
			result, remoteErr = remoteGetList(client, doctype, filters, fields, getLimit, getOrderBy)
		}

		if remoteErr != nil {
			return remoteErr
		}
		return printRemoteResult(result)
	}

	// Local mode
	benchPath, site, err := detectBenchAndSite()
	if err != nil {
		return err
	}

	executor := internalapi.NewExecutor(benchPath, site, apiUser)
	var result *internalapi.Result

	if name != "" {
		result, err = executor.GetDoc(doctype, name)
	} else {
		result, err = executor.GetList(doctype, filters, fields, getLimit, getOrderBy)
	}

	if err != nil {
		return err
	}

	return printResult(result)
}

// printResult formats and prints the result (local mode)
func printResult(result *internalapi.Result) error {
	if !result.Success {
		return fmt.Errorf("API error: %s", result.Error)
	}
	return printData(result.Data)
}

// printRemoteResult formats and prints the result (remote mode)
func printRemoteResult(result *RemoteResult) error {
	if !result.Success {
		return fmt.Errorf("API error: %s", result.Error)
	}
	return printData(result.Data)
}

// printData outputs data as JSON
func printData(data any) error {
	var output []byte
	var err error

	if apiRaw {
		// Compact JSON for piping
		output, err = json.Marshal(data)
	} else {
		// Pretty-printed JSON for humans
		output, err = json.MarshalIndent(data, "", "  ")
	}
	if err != nil {
		return err
	}

	fmt.Println(string(output))
	return nil
}
