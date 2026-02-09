package mcp

import (
	"encoding/json"
	"fmt"
	"testing"

	wegerrors "github.com/gavindsouza/weg/internal/errors"
	mcplib "github.com/mark3labs/mcp-go/mcp"
)

func TestErrorCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"not weg project", wegerrors.NotInProject("/tmp"), "not_weg_project"},
		{"config error", wegerrors.Config("weg.toml", "read", fmt.Errorf("not found")), "config"},
		{"state error", wegerrors.State("load", fmt.Errorf("corrupt")), "state"},
		{"validation error", wegerrors.Validation("code", "required"), "invalid_params"},
		{"api error 500", wegerrors.API(500, "internal server error", nil), "api"},
		{"api error 404", wegerrors.API(404, "not found", nil), "api"},
		{"not found error", wegerrors.NotFound("site", "mysite"), "not_found"},
		{"operation error", wegerrors.Operation("build", "compilation failed", nil), "operation"},
		{"usage error", wegerrors.Usage("invalid flag"), "invalid_params"},
		{"plain error", fmt.Errorf("something went wrong"), "internal"},
		{"wrapped typed error", fmt.Errorf("context: %w", wegerrors.NotInProject("/tmp")), "not_weg_project"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := errorCode(tt.err)
			if got != tt.want {
				t.Errorf("errorCode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func parseToolResultError(t *testing.T, result *mcplib.CallToolResult) mcpErrorInfo {
	t.Helper()
	if !result.IsError {
		t.Fatal("result should have IsError=true")
	}
	if len(result.Content) == 0 {
		t.Fatal("result has no content")
	}
	tc, ok := mcplib.AsTextContent(result.Content[0])
	if !ok {
		t.Fatalf("content[0] is not TextContent, got %T", result.Content[0])
	}
	var info mcpErrorInfo
	if err := json.Unmarshal([]byte(tc.Text), &info); err != nil {
		t.Fatalf("failed to parse error JSON %q: %v", tc.Text, err)
	}
	return info
}

func TestToolError(t *testing.T) {
	tests := []struct {
		name          string
		err           error
		wantCode      string
		wantRetryable bool
	}{
		{
			name:          "retryable api error",
			err:           wegerrors.API(500, "server error", nil),
			wantCode:      "api",
			wantRetryable: true,
		},
		{
			name:          "rate limited",
			err:           wegerrors.API(429, "too many requests", nil),
			wantCode:      "api",
			wantRetryable: true,
		},
		{
			name:          "non-retryable api error",
			err:           wegerrors.API(400, "bad request", nil),
			wantCode:      "api",
			wantRetryable: false,
		},
		{
			name:          "validation error not retryable",
			err:           wegerrors.Validation("doctype", "required"),
			wantCode:      "invalid_params",
			wantRetryable: false,
		},
		{
			name:          "plain error not retryable",
			err:           fmt.Errorf("binary not found"),
			wantCode:      "internal",
			wantRetryable: false,
		},
		{
			name:          "not found error",
			err:           wegerrors.NotFound("app", "erpnext"),
			wantCode:      "not_found",
			wantRetryable: false,
		},
		{
			name:          "operation wrapping retryable api error",
			err:           wegerrors.Operation("sync", "network failure", wegerrors.API(502, "bad gateway", nil)),
			wantCode:      "api",
			wantRetryable: true, // errors.As unwraps to find inner APIError
		},
		{
			name:          "operation error without wrapped type",
			err:           wegerrors.Operation("build", "compilation failed", fmt.Errorf("exit 1")),
			wantCode:      "operation",
			wantRetryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := toolError(tt.err)
			if err != nil {
				t.Fatalf("toolError() returned unexpected error: %v", err)
			}

			info := parseToolResultError(t, result)

			if info.Code != tt.wantCode {
				t.Errorf("code = %q, want %q", info.Code, tt.wantCode)
			}
			if info.Retryable != tt.wantRetryable {
				t.Errorf("retryable = %v, want %v", info.Retryable, tt.wantRetryable)
			}
			if info.Error != tt.err.Error() {
				t.Errorf("error = %q, want %q", info.Error, tt.err.Error())
			}
		})
	}
}

func TestToolParamError(t *testing.T) {
	err := fmt.Errorf("missing required parameter: code")
	result, rerr := toolParamError(err)
	if rerr != nil {
		t.Fatalf("toolParamError() returned unexpected error: %v", rerr)
	}

	info := parseToolResultError(t, result)

	if info.Code != "invalid_params" {
		t.Errorf("code = %q, want %q", info.Code, "invalid_params")
	}
	if info.Retryable {
		t.Error("param errors should not be retryable")
	}
	if info.Error != err.Error() {
		t.Errorf("error = %q, want %q", info.Error, err.Error())
	}
}
