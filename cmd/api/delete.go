package api

import (
	"fmt"
	"strings"

	internalapi "github.com/gavindsouza/weg/internal/api"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <doctype>/<name>",
	Short: "Delete a document",
	Long: `Delete a document from Frappe.

Examples:
  weg api delete User/test@example.com
  weg api delete "Sales Invoice/INV-001"`,
	Args:         cobra.ExactArgs(1),
	RunE:         runDelete,
	SilenceUsage: true,
}

func runDelete(cmd *cobra.Command, args []string) error {
	// Parse doctype/name
	arg := args[0]
	if !strings.Contains(arg, "/") {
		return fmt.Errorf("expected format: <doctype>/<name>")
	}

	parts := strings.SplitN(arg, "/", 2)
	doctype := parts[0]
	name := parts[1]

	// Remote mode
	if isRemoteMode() {
		if err := validateRemoteAuth(); err != nil {
			return err
		}

		client := NewRemoteClient(apiURL, apiKey, apiSecret)
		result, err := client.Delete(doctype, name)
		if err != nil {
			return err
		}

		if result.Success {
			fmt.Printf("Deleted %s/%s\n", doctype, name)
			return nil
		}
		return fmt.Errorf("API error: %s", result.Error)
	}

	// Local mode
	benchPath, site, err := detectBenchAndSite()
	if err != nil {
		return err
	}

	executor := internalapi.NewExecutor(benchPath, site, apiUser)
	result, err := executor.Delete(doctype, name)
	if err != nil {
		return err
	}

	if result.Success {
		fmt.Printf("Deleted %s/%s\n", doctype, name)
		return nil
	}

	return fmt.Errorf("API error: %s", result.Error)
}
