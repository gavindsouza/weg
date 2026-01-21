// Package errors provides typed errors for the weg CLI.
//
// It defines error types that can be checked with errors.Is/As,
// and provides exit code mapping for proper CLI exit status.
package errors

import (
	"errors"
	"fmt"

	"github.com/gavindsouza/weg/internal/output"
)

// Standard library errors functions re-exported for convenience.
var (
	Is     = errors.Is
	As     = errors.As
	Unwrap = errors.Unwrap
	Join   = errors.Join
	New    = errors.New
)

// NotWegProject is returned when a command is run outside a weg-managed project.
type NotWegProject struct {
	Path string
}

func (e *NotWegProject) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("not a weg-managed project: %s. Run 'weg init' first", e.Path)
	}
	return "not a weg-managed project. Run 'weg init' first"
}

// NotInProject returns a NotWegProject error.
func NotInProject(path string) error {
	return &NotWegProject{Path: path}
}

// ConfigError represents errors related to configuration files.
type ConfigError struct {
	File string // The config file (e.g., "weg.toml", "pyproject.toml")
	Op   string // The operation: "read", "write", "parse", "validate"
	Err  error  // The underlying error
}

func (e *ConfigError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("failed to %s %s: %v", e.Op, e.File, e.Err)
	}
	return fmt.Sprintf("failed to %s %s", e.Op, e.File)
}

func (e *ConfigError) Unwrap() error {
	return e.Err
}

// Config returns a ConfigError.
func Config(file, op string, err error) error {
	return &ConfigError{File: file, Op: op, Err: err}
}

// StateError represents errors related to state file operations.
type StateError struct {
	Op  string // The operation: "load", "save", "validate"
	Err error  // The underlying error
}

func (e *StateError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("failed to %s state: %v", e.Op, e.Err)
	}
	return fmt.Sprintf("failed to %s state", e.Op)
}

func (e *StateError) Unwrap() error {
	return e.Err
}

// State returns a StateError.
func State(op string, err error) error {
	return &StateError{Op: op, Err: err}
}

// ValidationError represents input validation errors.
type ValidationError struct {
	Field   string // The field that failed validation
	Message string // Description of the validation failure
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("invalid %s: %s", e.Field, e.Message)
	}
	return e.Message
}

// Validation returns a ValidationError.
func Validation(field, message string) error {
	return &ValidationError{Field: field, Message: message}
}

// APIError represents errors from API/network operations.
type APIError struct {
	StatusCode int    // HTTP status code (0 if not applicable)
	Message    string // Error message
	Err        error  // The underlying error
}

func (e *APIError) Error() string {
	if e.StatusCode > 0 {
		if e.Err != nil {
			return fmt.Sprintf("API error (HTTP %d): %s: %v", e.StatusCode, e.Message, e.Err)
		}
		return fmt.Sprintf("API error (HTTP %d): %s", e.StatusCode, e.Message)
	}
	if e.Err != nil {
		return fmt.Sprintf("API error: %s: %v", e.Message, e.Err)
	}
	return fmt.Sprintf("API error: %s", e.Message)
}

func (e *APIError) Unwrap() error {
	return e.Err
}

// API returns an APIError.
func API(statusCode int, message string, err error) error {
	return &APIError{StatusCode: statusCode, Message: message, Err: err}
}

// NotFoundError represents a resource not found error.
type NotFoundError struct {
	Resource string // Type of resource (e.g., "site", "app", "file")
	Name     string // Name/identifier of the resource
}

func (e *NotFoundError) Error() string {
	if e.Name != "" {
		return fmt.Sprintf("%s %q not found", e.Resource, e.Name)
	}
	return fmt.Sprintf("%s not found", e.Resource)
}

// NotFound returns a NotFoundError.
func NotFound(resource, name string) error {
	return &NotFoundError{Resource: resource, Name: name}
}

// OperationError represents errors during operations (sync, build, etc.).
type OperationError struct {
	Operation string // The operation name
	Message   string // Error description
	Err       error  // The underlying error
}

func (e *OperationError) Error() string {
	if e.Err != nil {
		if e.Message != "" {
			return fmt.Sprintf("%s failed: %s: %v", e.Operation, e.Message, e.Err)
		}
		return fmt.Sprintf("%s failed: %v", e.Operation, e.Err)
	}
	if e.Message != "" {
		return fmt.Sprintf("%s failed: %s", e.Operation, e.Message)
	}
	return fmt.Sprintf("%s failed", e.Operation)
}

func (e *OperationError) Unwrap() error {
	return e.Err
}

// Operation returns an OperationError.
func Operation(operation, message string, err error) error {
	return &OperationError{Operation: operation, Message: message, Err: err}
}

// UsageError represents incorrect command usage.
type UsageError struct {
	Message string
}

func (e *UsageError) Error() string {
	return e.Message
}

// Usage returns a UsageError.
func Usage(message string) error {
	return &UsageError{Message: message}
}

// Usagef returns a formatted UsageError.
func Usagef(format string, args ...any) error {
	return &UsageError{Message: fmt.Sprintf(format, args...)}
}

// Exit codes for CLI operations.
// These follow common Unix conventions where possible.
const (
	ExitSuccess     = 0   // Successful execution
	ExitGeneric     = 1   // Generic error
	ExitUsage       = 2   // Invalid usage/arguments
	ExitConfig      = 3   // Configuration error
	ExitState       = 4   // State file error
	ExitNetwork     = 5   // Network/API error
	ExitNotFound    = 6   // Resource not found
	ExitPermission  = 7   // Permission denied
	ExitInterrupted = 130 // Interrupted (Ctrl+C)
)

// ExitCode returns the appropriate exit code for an error.
// Returns ExitSuccess (0) for nil errors.
func ExitCode(err error) int {
	if err == nil {
		return ExitSuccess
	}

	// Check for specific error types
	var notWegProject *NotWegProject
	if As(err, &notWegProject) {
		return ExitConfig
	}

	var configErr *ConfigError
	if As(err, &configErr) {
		return ExitConfig
	}

	var stateErr *StateError
	if As(err, &stateErr) {
		return ExitState
	}

	var validationErr *ValidationError
	if As(err, &validationErr) {
		return ExitUsage
	}

	var apiErr *APIError
	if As(err, &apiErr) {
		return ExitNetwork
	}

	var notFoundErr *NotFoundError
	if As(err, &notFoundErr) {
		return ExitNotFound
	}

	var opErr *OperationError
	if As(err, &opErr) {
		// Check if underlying error has a more specific code
		if opErr.Err != nil {
			code := ExitCode(opErr.Err)
			if code != ExitGeneric {
				return code
			}
		}
		return ExitGeneric
	}

	var usageErr *UsageError
	if As(err, &usageErr) {
		return ExitUsage
	}

	return ExitGeneric
}

// Print prints an error message to stderr with the error symbol.
// Does nothing if err is nil.
func Print(err error) {
	if err == nil {
		return
	}
	output.Errorf("%s", err.Error())
}

// Printf prints a formatted error message to stderr.
func Printf(format string, args ...any) {
	output.Errorf(format, args...)
}

// PrintWithHint prints an error with an optional hint for how to resolve it.
func PrintWithHint(err error, hint string) {
	if err == nil {
		return
	}
	output.Errorf("%s", err.Error())
	if hint != "" {
		fmt.Fprintf(output.ErrWriter, "Hint: %s\n", hint)
	}
}

// Exit prints the error (if non-nil) and returns the appropriate exit code.
// Use with os.Exit: os.Exit(errors.Exit(err))
func Exit(err error) int {
	Print(err)
	return ExitCode(err)
}

// IsUserError returns true if the error is a user-correctable error
// (usage, validation, not found) rather than an internal/system error.
func IsUserError(err error) bool {
	if err == nil {
		return false
	}

	var validationErr *ValidationError
	var usageErr *UsageError
	var notFoundErr *NotFoundError
	var notWegProject *NotWegProject

	return As(err, &validationErr) ||
		As(err, &usageErr) ||
		As(err, &notFoundErr) ||
		As(err, &notWegProject)
}

// IsRetryable returns true if the error might succeed on retry
// (typically network errors).
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	var apiErr *APIError
	if As(err, &apiErr) {
		// Server errors (5xx) are typically retryable
		// Rate limiting (429) is retryable
		return apiErr.StatusCode >= 500 || apiErr.StatusCode == 429
	}

	return false
}
