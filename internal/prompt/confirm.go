package prompt

import (
	"fmt"
	"strings"
)

// Confirm asks a yes/no question with default No.
// Returns true if user confirms.
// Respects AssumeYes flag.
//
// Example:
//
//	if !prompt.Confirm("Delete site %s?", siteName) {
//	    return nil // cancelled
//	}
//
// Output: "Delete site mysite? [y/N]: "
func Confirm(format string, args ...any) bool {
	return ConfirmDefault(false, format, args...)
}

// ConfirmDanger asks confirmation for destructive actions.
// Includes "This cannot be undone." warning.
// Returns true if user confirms.
// Respects AssumeYes flag.
//
// Example:
//
//	if !prompt.ConfirmDanger("Remove app %s?", appName) {
//	    return nil // cancelled
//	}
//
// Output: "Remove app myapp? This cannot be undone. [y/N]: "
func ConfirmDanger(format string, args ...any) bool {
	message := fmt.Sprintf(format, args...)
	return ConfirmDefault(false, "%s This cannot be undone.", message)
}

// ConfirmDefault asks a yes/no question with specified default.
// defaultYes=true makes Enter key confirm (Y/n)
// defaultYes=false makes Enter key cancel (y/N)
//
// Example:
//
//	// Default yes - useful for clone confirmations
//	if !prompt.ConfirmDefault(true, "Install dependencies?") {
//	    // user said no
//	}
//
// Output: "Install dependencies? [Y/n]: "
func ConfirmDefault(defaultYes bool, format string, args ...any) bool {
	// Honor AssumeYes flag
	if AssumeYes {
		return true
	}

	message := fmt.Sprintf(format, args...)

	// Format prompt with appropriate default indicator
	var promptSuffix string
	if defaultYes {
		promptSuffix = "[Y/n]"
	} else {
		promptSuffix = "[y/N]"
	}

	write("%s %s: ", message, promptSuffix)

	// Read response
	response, err := readLine()
	if err != nil {
		// On read error (EOF, etc.), use default
		return defaultYes
	}

	// Parse response
	response = strings.ToLower(strings.TrimSpace(response))
	switch response {
	case "y", "yes":
		return true
	case "n", "no":
		return false
	case "":
		return defaultYes
	default:
		// Invalid input - ask again
		writeln("Please answer 'y' or 'n'.")
		return ConfirmDefault(defaultYes, format, args...)
	}
}

// MustConfirm is like Confirm but panics if the user declines.
// Use sparingly - prefer returning errors for better UX.
func MustConfirm(format string, args ...any) {
	if !Confirm(format, args...) {
		panic("user declined confirmation")
	}
}
