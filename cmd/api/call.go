package api

import (
	"encoding/json"
	"fmt"
	"strings"

	internalapi "github.com/gavindsouza/weg/internal/api"
	"github.com/spf13/cobra"
)

var callArgs string

var callCmd = &cobra.Command{
	Use:   "call <method> [key=value...]",
	Short: "Call a Frappe method",
	Long: `Call any whitelisted Frappe method with arguments.

Arguments can be passed as key=value pairs after the method name,
or as JSON using the --args flag.

Examples:
  weg api call frappe.ping
  weg api call frappe.client.get doctype=User name=Administrator
  weg api call frappe.client.get_count doctype=User
  weg api call myapp.api.custom_function arg1=value1 arg2=value2
  weg api call myapp.api.create --args '{"customer":"CUST-001","items":[...]}'`,
	Args:         cobra.MinimumNArgs(1),
	RunE:         runCall,
	SilenceUsage: true,
}

func init() {
	callCmd.Flags().StringVar(&callArgs, "args", "", "JSON object of arguments")
}

func runCall(cmd *cobra.Command, args []string) error {
	benchPath, site, err := detectBenchAndSite()
	if err != nil {
		return err
	}

	executor := internalapi.NewExecutor(benchPath, site, apiUser)

	method := args[0]
	kwargs := make(map[string]interface{})

	// Parse --args JSON if provided
	if callArgs != "" {
		if err := json.Unmarshal([]byte(callArgs), &kwargs); err != nil {
			return fmt.Errorf("invalid --args JSON: %w", err)
		}
	}

	// Parse key=value arguments
	for _, arg := range args[1:] {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid argument format: %s (expected key=value)", arg)
		}
		key := parts[0]
		value := parts[1]

		// Try to parse as JSON for complex values
		var jsonValue interface{}
		if err := json.Unmarshal([]byte(value), &jsonValue); err == nil {
			kwargs[key] = jsonValue
		} else {
			kwargs[key] = value
		}
	}

	result, err := executor.Call(method, kwargs)
	if err != nil {
		return err
	}

	return printResult(result)
}
