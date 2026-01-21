package prompt

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

// Password prompts for a password with masked input.
// Falls back to plain input if not a terminal.
//
// Example:
//
//	password, err := prompt.Password("Enter admin password: ")
//	if err != nil {
//	    return err
//	}
func Password(promptText string) (string, error) {
	write("%s", promptText)

	// Try to read with masked input
	if f, ok := Reader.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		password, err := term.ReadPassword(int(f.Fd()))
		writeln("") // newline after masked input
		if err != nil {
			return "", fmt.Errorf("failed to read password: %w", err)
		}
		return string(password), nil
	}

	// Fall back to plain input for non-TTY
	password, err := readLine()
	if err != nil {
		return "", fmt.Errorf("failed to read password: %w", err)
	}
	return password, nil
}

// PasswordConfirm prompts for a password with confirmation.
// Returns error if passwords don't match.
//
// Example:
//
//	password, err := prompt.PasswordConfirm("New password for %s: ", username)
//	if err != nil {
//	    return err
//	}
func PasswordConfirm(format string, args ...any) (string, error) {
	promptText := fmt.Sprintf(format, args...)

	password, err := Password(promptText)
	if err != nil {
		return "", err
	}

	confirm, err := Password("Confirm password: ")
	if err != nil {
		return "", err
	}

	if password != confirm {
		return "", fmt.Errorf("passwords do not match")
	}

	return password, nil
}

// PasswordWithDefault prompts for password, returns default if empty.
// Useful for optional passwords where a sensible default exists.
//
// Example:
//
//	password, err := prompt.PasswordWithDefault("admin", "Admin password (default: admin): ")
func PasswordWithDefault(defaultVal, promptText string) (string, error) {
	password, err := Password(promptText)
	if err != nil {
		return "", err
	}

	if password == "" {
		return defaultVal, nil
	}
	return password, nil
}
