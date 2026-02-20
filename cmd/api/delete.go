package api

import (
	"strings"

	internalapi "github.com/gavindsouza/weg/internal/api"
	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/remote"
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
		return wegerrors.Usage("expected format: <doctype>/<name>")
	}

	parts := strings.SplitN(arg, "/", 2)
	doctype := parts[0]
	name := parts[1]

	// Remote mode
	if isRemoteMode() {
		key, secret, err := resolveRemoteCredentials()
		if err != nil {
			return err
		}

		client := remote.NewClient(apiURL, key, secret)
		result, err := remoteDelete(client, doctype, name)
		if err != nil {
			return err
		}

		if result.Success {
			output.Printf("Deleted %s/%s", doctype, name)
			return nil
		}
		return wegerrors.API(0, result.Error, nil)
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
		output.Printf("Deleted %s/%s", doctype, name)
		return nil
	}

	return wegerrors.API(0, result.Error, nil)
}
