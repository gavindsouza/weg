package errors

import (
	"bytes"
	stderrors "errors"
	"testing"

	"github.com/gavindsouza/weg/internal/output"
)

func TestNotWegProject(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"with path", "/some/path", "not a weg-managed project: /some/path. Run 'weg init' first"},
		{"without path", "", "not a weg-managed project. Run 'weg init' first"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NotInProject(tt.path)
			if err.Error() != tt.expected {
				t.Errorf("got %q, want %q", err.Error(), tt.expected)
			}
		})
	}
}

func TestConfigError(t *testing.T) {
	tests := []struct {
		name     string
		file     string
		op       string
		err      error
		expected string
	}{
		{"with underlying error", "weg.toml", "parse", stderrors.New("invalid syntax"), "failed to parse weg.toml: invalid syntax"},
		{"without underlying error", "weg.toml", "read", nil, "failed to read weg.toml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Config(tt.file, tt.op, tt.err)
			if err.Error() != tt.expected {
				t.Errorf("got %q, want %q", err.Error(), tt.expected)
			}

			// Test Unwrap
			var configErr *ConfigError
			if As(err, &configErr) {
				if configErr.Unwrap() != tt.err {
					t.Error("Unwrap returned wrong error")
				}
			}
		})
	}
}

func TestStateError(t *testing.T) {
	tests := []struct {
		name     string
		op       string
		err      error
		expected string
	}{
		{"with underlying error", "load", stderrors.New("file not found"), "failed to load state: file not found"},
		{"without underlying error", "save", nil, "failed to save state"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := State(tt.op, tt.err)
			if err.Error() != tt.expected {
				t.Errorf("got %q, want %q", err.Error(), tt.expected)
			}
		})
	}
}

func TestValidationError(t *testing.T) {
	tests := []struct {
		name     string
		field    string
		message  string
		expected string
	}{
		{"with field", "email", "must be valid email", "invalid email: must be valid email"},
		{"without field", "", "input is required", "input is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validation(tt.field, tt.message)
			if err.Error() != tt.expected {
				t.Errorf("got %q, want %q", err.Error(), tt.expected)
			}
		})
	}
}

func TestAPIError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		message    string
		err        error
		expected   string
	}{
		{"with status and error", 500, "server error", stderrors.New("connection reset"), "API error (HTTP 500): server error: connection reset"},
		{"with status only", 404, "not found", nil, "API error (HTTP 404): not found"},
		{"without status with error", 0, "network error", stderrors.New("timeout"), "API error: network error: timeout"},
		{"without status or error", 0, "failed", nil, "API error: failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := API(tt.statusCode, tt.message, tt.err)
			if err.Error() != tt.expected {
				t.Errorf("got %q, want %q", err.Error(), tt.expected)
			}
		})
	}
}

func TestNotFoundError(t *testing.T) {
	tests := []struct {
		name     string
		resource string
		resName  string
		expected string
	}{
		{"with name", "site", "mysite", `site "mysite" not found`},
		{"without name", "app", "", "app not found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NotFound(tt.resource, tt.resName)
			if err.Error() != tt.expected {
				t.Errorf("got %q, want %q", err.Error(), tt.expected)
			}
		})
	}
}

func TestOperationError(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		message   string
		err       error
		expected  string
	}{
		{"with message and error", "sync", "failed to connect", stderrors.New("timeout"), "sync failed: failed to connect: timeout"},
		{"with error only", "build", "", stderrors.New("compilation error"), "build failed: compilation error"},
		{"with message only", "deploy", "missing config", nil, "deploy failed: missing config"},
		{"neither", "cleanup", "", nil, "cleanup failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Operation(tt.operation, tt.message, tt.err)
			if err.Error() != tt.expected {
				t.Errorf("got %q, want %q", err.Error(), tt.expected)
			}
		})
	}
}

func TestUsageError(t *testing.T) {
	err := Usage("missing required argument")
	if err.Error() != "missing required argument" {
		t.Errorf("unexpected error message: %s", err.Error())
	}

	err = Usagef("unknown flag: %s", "--invalid")
	if err.Error() != "unknown flag: --invalid" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestExitCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{"nil error", nil, ExitSuccess},
		{"NotWegProject", NotInProject("/path"), ExitConfig},
		{"ConfigError", Config("weg.toml", "parse", nil), ExitConfig},
		{"StateError", State("load", nil), ExitState},
		{"ValidationError", Validation("field", "invalid"), ExitUsage},
		{"APIError", API(500, "error", nil), ExitNetwork},
		{"NotFoundError", NotFound("site", "name"), ExitNotFound},
		{"UsageError", Usage("invalid"), ExitUsage},
		{"generic error", stderrors.New("unknown"), ExitGeneric},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := ExitCode(tt.err)
			if code != tt.expected {
				t.Errorf("got %d, want %d", code, tt.expected)
			}
		})
	}
}

func TestExitCode_OperationError_Unwrapped(t *testing.T) {
	// OperationError should inherit exit code from wrapped error
	apiErr := API(500, "server error", nil)
	opErr := Operation("sync", "api call failed", apiErr)

	code := ExitCode(opErr)
	if code != ExitNetwork {
		t.Errorf("expected ExitNetwork (%d), got %d", ExitNetwork, code)
	}
}

func TestIsUserError(t *testing.T) {
	userErrors := []error{
		Validation("field", "invalid"),
		Usage("bad args"),
		NotFound("site", "name"),
		NotInProject("/path"),
	}

	for _, err := range userErrors {
		if !IsUserError(err) {
			t.Errorf("expected %T to be user error", err)
		}
	}

	nonUserErrors := []error{
		Config("file", "op", nil),
		State("op", nil),
		API(500, "error", nil),
		stderrors.New("generic"),
	}

	for _, err := range nonUserErrors {
		if IsUserError(err) {
			t.Errorf("expected %T to not be user error", err)
		}
	}

	// nil should not be user error
	if IsUserError(nil) {
		t.Error("nil should not be user error")
	}
}

func TestIsRetryable(t *testing.T) {
	retryable := []error{
		API(500, "server error", nil),
		API(502, "bad gateway", nil),
		API(503, "service unavailable", nil),
		API(429, "rate limited", nil),
	}

	for _, err := range retryable {
		if !IsRetryable(err) {
			t.Errorf("expected %v to be retryable", err)
		}
	}

	notRetryable := []error{
		API(400, "bad request", nil),
		API(401, "unauthorized", nil),
		API(404, "not found", nil),
		stderrors.New("generic"),
		nil,
	}

	for _, err := range notRetryable {
		if IsRetryable(err) {
			t.Errorf("expected %v to not be retryable", err)
		}
	}
}

func TestPrint(t *testing.T) {
	// Save and restore state
	oldWriter := output.ErrWriter
	defer func() { output.ErrWriter = oldWriter }()

	var buf bytes.Buffer
	output.ErrWriter = &buf

	// Test nil error (should not print)
	Print(nil)
	if buf.String() != "" {
		t.Errorf("Print(nil) should not output, got %q", buf.String())
	}

	// Test actual error
	buf.Reset()
	Print(stderrors.New("test error"))
	if buf.String() == "" {
		t.Error("Print(err) should output something")
	}
}

func TestPrintWithHint(t *testing.T) {
	// Save and restore state
	oldWriter := output.ErrWriter
	defer func() { output.ErrWriter = oldWriter }()

	var buf bytes.Buffer
	output.ErrWriter = &buf

	PrintWithHint(stderrors.New("test error"), "try running 'weg init'")
	out := buf.String()

	if out == "" {
		t.Error("PrintWithHint should output something")
	}
	if !bytes.Contains(buf.Bytes(), []byte("Hint:")) {
		t.Error("PrintWithHint should include hint")
	}
}

func TestExit(t *testing.T) {
	// Save and restore state
	oldWriter := output.ErrWriter
	defer func() { output.ErrWriter = oldWriter }()

	var buf bytes.Buffer
	output.ErrWriter = &buf

	code := Exit(Validation("field", "invalid"))
	if code != ExitUsage {
		t.Errorf("expected ExitUsage (%d), got %d", ExitUsage, code)
	}
	if buf.String() == "" {
		t.Error("Exit should print error")
	}
}

func TestReExportedFunctions(t *testing.T) {
	// Test that standard library functions are properly re-exported
	err1 := New("error 1")
	err2 := New("error 2")

	// Test Join
	joined := Join(err1, err2)
	if joined == nil {
		t.Error("Join should return non-nil error")
	}

	// Test Is
	if !Is(err1, err1) {
		t.Error("Is should return true for same error")
	}

	// Test Unwrap (with a wrapped error)
	wrapped := Config("file", "op", err1)
	var configErr *ConfigError
	if !As(wrapped, &configErr) {
		t.Error("As should find ConfigError")
	}
	if Unwrap(wrapped) != err1 {
		t.Error("Unwrap should return wrapped error")
	}
}
