package prompt

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

// mockReader creates a reader from a string for testing
func mockReader(input string) io.Reader {
	return strings.NewReader(input)
}

// setupTest configures prompt for testing and returns cleanup function
func setupTest(input string) func() {
	oldReader := Reader
	oldWriter := Writer
	oldAssumeYes := AssumeYes

	Reader = mockReader(input)
	ResetReader() // Clear cached bufio.Reader
	Writer = io.Discard // suppress output during tests

	return func() {
		Reader = oldReader
		ResetReader()
		Writer = oldWriter
		AssumeYes = oldAssumeYes
	}
}

func TestConfirm_Yes(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"y\n", true},
		{"Y\n", true},
		{"yes\n", true},
		{"YES\n", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			cleanup := setupTest(tt.input)
			defer cleanup()

			result := Confirm("Delete?")
			if result != tt.expected {
				t.Errorf("Confirm() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestConfirm_No(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"n\n", false},
		{"N\n", false},
		{"no\n", false},
		{"NO\n", false},
		{"\n", false}, // empty = default (no)
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			cleanup := setupTest(tt.input)
			defer cleanup()

			result := Confirm("Delete?")
			if result != tt.expected {
				t.Errorf("Confirm() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestConfirm_AssumeYes(t *testing.T) {
	cleanup := setupTest("n\n") // would be no, but AssumeYes overrides
	defer cleanup()

	AssumeYes = true
	result := Confirm("Delete?")
	if !result {
		t.Error("Confirm should return true when AssumeYes is set")
	}
}

func TestConfirmDefault_Yes(t *testing.T) {
	cleanup := setupTest("\n") // empty input uses default
	defer cleanup()

	// Default yes: empty should return true
	result := ConfirmDefault(true, "Install?")
	if !result {
		t.Error("ConfirmDefault(true, ...) should return true on empty input")
	}
}

func TestConfirmDefault_No(t *testing.T) {
	cleanup := setupTest("\n") // empty input uses default
	defer cleanup()

	// Default no: empty should return false
	result := ConfirmDefault(false, "Delete?")
	if result {
		t.Error("ConfirmDefault(false, ...) should return false on empty input")
	}
}

func TestConfirmDanger(t *testing.T) {
	cleanup := setupTest("y\n")
	defer cleanup()

	// Should include warning text
	var buf bytes.Buffer
	Writer = &buf

	result := ConfirmDanger("Delete app %s?", "myapp")
	if !result {
		t.Error("ConfirmDanger should return true on 'y'")
	}

	output := buf.String()
	if !strings.Contains(output, "This cannot be undone") {
		t.Error("ConfirmDanger should include warning text")
	}
	if !strings.Contains(output, "myapp") {
		t.Error("ConfirmDanger should include formatted args")
	}
}

func TestInput(t *testing.T) {
	cleanup := setupTest("hello world\n")
	defer cleanup()

	result, err := Input("Enter value: ")
	if err != nil {
		t.Fatalf("Input() error = %v", err)
	}
	if result != "hello world" {
		t.Errorf("Input() = %q, want %q", result, "hello world")
	}
}

func TestInputWithDefault_Empty(t *testing.T) {
	cleanup := setupTest("\n")
	defer cleanup()

	result, err := InputWithDefault("default", "Value: ")
	if err != nil {
		t.Fatalf("InputWithDefault() error = %v", err)
	}
	if result != "default" {
		t.Errorf("InputWithDefault() = %q, want %q", result, "default")
	}
}

func TestInputWithDefault_Value(t *testing.T) {
	cleanup := setupTest("custom\n")
	defer cleanup()

	result, err := InputWithDefault("default", "Value: ")
	if err != nil {
		t.Fatalf("InputWithDefault() error = %v", err)
	}
	if result != "custom" {
		t.Errorf("InputWithDefault() = %q, want %q", result, "custom")
	}
}

func TestInputRequired(t *testing.T) {
	// First empty, then valid
	cleanup := setupTest("\nvalue\n")
	defer cleanup()

	result, err := InputRequired("Name: ")
	if err != nil {
		t.Fatalf("InputRequired() error = %v", err)
	}
	if result != "value" {
		t.Errorf("InputRequired() = %q, want %q", result, "value")
	}
}

func TestPassword(t *testing.T) {
	// Non-TTY fallback (plain input)
	cleanup := setupTest("secret123\n")
	defer cleanup()

	result, err := Password("Password: ")
	if err != nil {
		t.Fatalf("Password() error = %v", err)
	}
	if result != "secret123" {
		t.Errorf("Password() = %q, want %q", result, "secret123")
	}
}

func TestPasswordWithDefault_Empty(t *testing.T) {
	cleanup := setupTest("\n")
	defer cleanup()

	result, err := PasswordWithDefault("admin", "Password: ")
	if err != nil {
		t.Fatalf("PasswordWithDefault() error = %v", err)
	}
	if result != "admin" {
		t.Errorf("PasswordWithDefault() = %q, want %q", result, "admin")
	}
}

func TestPasswordWithDefault_Value(t *testing.T) {
	cleanup := setupTest("custom\n")
	defer cleanup()

	result, err := PasswordWithDefault("admin", "Password: ")
	if err != nil {
		t.Fatalf("PasswordWithDefault() error = %v", err)
	}
	if result != "custom" {
		t.Errorf("PasswordWithDefault() = %q, want %q", result, "custom")
	}
}

func TestPasswordConfirm_Match(t *testing.T) {
	cleanup := setupTest("secret\nsecret\n")
	defer cleanup()

	result, err := PasswordConfirm("Password: ")
	if err != nil {
		t.Fatalf("PasswordConfirm() error = %v", err)
	}
	if result != "secret" {
		t.Errorf("PasswordConfirm() = %q, want %q", result, "secret")
	}
}

func TestPasswordConfirm_Mismatch(t *testing.T) {
	cleanup := setupTest("secret1\nsecret2\n")
	defer cleanup()

	_, err := PasswordConfirm("Password: ")
	if err == nil {
		t.Error("PasswordConfirm() should error on mismatch")
	}
	if !strings.Contains(err.Error(), "do not match") {
		t.Errorf("error should mention mismatch: %v", err)
	}
}

func TestIsTTY_NonTTY(t *testing.T) {
	cleanup := setupTest("")
	defer cleanup()

	// strings.Reader is not a TTY
	if IsTTY() {
		t.Error("IsTTY() should return false for non-file reader")
	}
}
