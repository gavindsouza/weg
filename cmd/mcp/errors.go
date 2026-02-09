package mcp

import (
	"encoding/json"

	wegerrors "github.com/gavindsouza/weg/internal/errors"
	mcplib "github.com/mark3labs/mcp-go/mcp"
)

// mcpErrorInfo is the structured error payload returned to MCP clients.
// AI consumers can use "code" and "retryable" to decide how to handle failures.
type mcpErrorInfo struct {
	Error     string `json:"error"`
	Code      string `json:"code"`
	Retryable bool   `json:"retryable"`
}

// errorCode classifies an error into a machine-readable code string.
func errorCode(err error) string {
	var notWeg *wegerrors.NotWegProject
	if wegerrors.As(err, &notWeg) {
		return "not_weg_project"
	}

	var configErr *wegerrors.ConfigError
	if wegerrors.As(err, &configErr) {
		return "config"
	}

	var stateErr *wegerrors.StateError
	if wegerrors.As(err, &stateErr) {
		return "state"
	}

	var validationErr *wegerrors.ValidationError
	if wegerrors.As(err, &validationErr) {
		return "invalid_params"
	}

	var apiErr *wegerrors.APIError
	if wegerrors.As(err, &apiErr) {
		return "api"
	}

	var notFoundErr *wegerrors.NotFoundError
	if wegerrors.As(err, &notFoundErr) {
		return "not_found"
	}

	var opErr *wegerrors.OperationError
	if wegerrors.As(err, &opErr) {
		return "operation"
	}

	var usageErr *wegerrors.UsageError
	if wegerrors.As(err, &usageErr) {
		return "invalid_params"
	}

	return "internal"
}

// toolError returns a structured MCP error result with classification.
func toolError(err error) (*mcplib.CallToolResult, error) {
	info := mcpErrorInfo{
		Error:     err.Error(),
		Code:      errorCode(err),
		Retryable: wegerrors.IsRetryable(err),
	}
	data, _ := json.Marshal(info)
	return mcplib.NewToolResultError(string(data)), nil
}

// toolParamError returns a structured MCP error for parameter validation failures.
func toolParamError(err error) (*mcplib.CallToolResult, error) {
	info := mcpErrorInfo{
		Error:     err.Error(),
		Code:      "invalid_params",
		Retryable: false,
	}
	data, _ := json.Marshal(info)
	return mcplib.NewToolResultError(string(data)), nil
}
