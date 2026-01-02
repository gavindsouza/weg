package cloud

import (
	"fmt"
	"os"

	"github.com/gavindsouza/weg/internal/cloud"
	"github.com/spf13/cobra"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out from Frappe Cloud",
	Long:  `Remove stored Frappe Cloud credentials.`,
	RunE:  runLogout,
}

func runLogout(cmd *cobra.Command, args []string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	if err := cloud.RemoveCredentials(homeDir); err != nil {
		return fmt.Errorf("failed to remove credentials: %w", err)
	}

	fmt.Println("Logged out from Frappe Cloud")
	return nil
}
