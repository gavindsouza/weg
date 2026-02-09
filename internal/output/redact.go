package output

import (
	"net/http"
	"regexp"
	"strings"
)

// Redactor handles secret masking in debug output.
// Redaction cannot be disabled - this is security by design.
type Redactor struct {
	// secretFields are field names that should always be redacted
	secretFields map[string]bool

	// patterns are regex patterns to detect secrets in values
	patterns []*regexp.Regexp
}

// defaultRedactor is the global redactor instance.
var defaultRedactor = newDefaultRedactor()

// newDefaultRedactor creates a redactor with standard secret patterns.
func newDefaultRedactor() *Redactor {
	r := &Redactor{
		secretFields: map[string]bool{
			// Common secret field names (lowercase for case-insensitive matching)
			"password":      true,
			"passwd":        true,
			"pwd":           true,
			"secret":        true,
			"token":         true,
			"api_key":       true,
			"apikey":        true,
			"api_secret":    true,
			"apisecret":     true,
			"key":           true,
			"auth":          true,
			"authorization": true,
			"credential":    true,
			"credentials":   true,
			"private_key":   true,
			"privatekey":    true,
			"access_token":  true,
			"accesstoken":   true,
			"refresh_token": true,
			"refreshtoken":  true,
			"bearer":        true,
			"session":       true,
			"cookie":        true,
		},
	}

	// Patterns to detect secrets in string values
	r.patterns = []*regexp.Regexp{
		// Bearer tokens
		regexp.MustCompile(`(?i)bearer\s+[a-zA-Z0-9_\-\.]+`),
		// Basic auth
		regexp.MustCompile(`(?i)basic\s+[a-zA-Z0-9+/=]+`),
		// API key patterns (key:secret format)
		regexp.MustCompile(`[a-zA-Z0-9]{16,}:[a-zA-Z0-9]{16,}`),
		// JWT tokens (eyJ prefix = base64 of {"
		regexp.MustCompile(`eyJ[a-zA-Z0-9_-]{10,}\.[a-zA-Z0-9_-]{10,}\.[a-zA-Z0-9_-]{10,}`),
		// AWS access key IDs
		regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
		// GitHub tokens (classic PAT, fine-grained PAT, OAuth)
		regexp.MustCompile(`gh[ps]_[a-zA-Z0-9]{36,}`),
		regexp.MustCompile(`github_pat_[a-zA-Z0-9_]{22,}`),
		regexp.MustCompile(`gho_[a-zA-Z0-9]{36,}`),
	}

	return r
}

// IsSecretField checks if a field name indicates a secret value.
// Uses substring matching so "db_password_hash" matches "password".
func (r *Redactor) IsSecretField(name string) bool {
	lower := strings.ToLower(name)
	if r.secretFields[lower] {
		return true
	}
	for field := range r.secretFields {
		if strings.Contains(lower, field) {
			return true
		}
	}
	return false
}

// Redact masks a secret value.
// Shows first 3 and last 3 chars for values >= 8 chars, otherwise just ***.
func (r *Redactor) Redact(value string) string {
	if value == "" {
		return ""
	}

	if len(value) < 8 {
		return "***"
	}

	return value[:3] + "***" + value[len(value)-3:]
}

// RedactString masks any detected secrets in a string.
func (r *Redactor) RedactString(s string) string {
	result := s

	// Apply regex patterns
	for _, pattern := range r.patterns {
		result = pattern.ReplaceAllStringFunc(result, func(match string) string {
			return r.Redact(match)
		})
	}

	return result
}

// RedactMap masks sensitive values in a map.
// Keys matching secret field names have their values redacted.
func (r *Redactor) RedactMap(m map[string]any) map[string]any {
	result := make(map[string]any, len(m))

	for k, v := range m {
		if r.IsSecretField(k) {
			if s, ok := v.(string); ok {
				result[k] = r.Redact(s)
			} else {
				result[k] = "***"
			}
		} else if nested, ok := v.(map[string]any); ok {
			result[k] = r.RedactMap(nested)
		} else if s, ok := v.(string); ok {
			result[k] = r.RedactString(s)
		} else {
			result[k] = v
		}
	}

	return result
}

// RedactHeaders masks sensitive HTTP headers.
func (r *Redactor) RedactHeaders(h http.Header) http.Header {
	result := make(http.Header, len(h))

	sensitiveHeaders := map[string]bool{
		"Authorization":   true,
		"X-Api-Key":       true,
		"X-Api-Secret":    true,
		"Cookie":          true,
		"Set-Cookie":      true,
		"X-Auth-Token":    true,
		"X-Access-Token":  true,
		"X-Refresh-Token": true,
	}

	for k, values := range h {
		if sensitiveHeaders[k] {
			redacted := make([]string, len(values))
			for i, v := range values {
				redacted[i] = r.Redact(v)
			}
			result[k] = redacted
		} else {
			result[k] = values
		}
	}

	return result
}

// RedactJSON redacts secrets from a JSON-like structure.
func (r *Redactor) RedactJSON(data []byte) []byte {
	// Simple approach: redact known patterns in the raw JSON
	result := string(data)
	result = r.RedactString(result)
	return []byte(result)
}

// Package-level convenience functions using the default redactor

// Redact masks a secret value using the default redactor.
func Redact(value string) string {
	return defaultRedactor.Redact(value)
}

// RedactString masks secrets in a string using the default redactor.
func RedactString(s string) string {
	return defaultRedactor.RedactString(s)
}

// RedactMap masks secrets in a map using the default redactor.
func RedactMap(m map[string]any) map[string]any {
	return defaultRedactor.RedactMap(m)
}

// RedactHeaders masks sensitive HTTP headers using the default redactor.
func RedactHeaders(h http.Header) http.Header {
	return defaultRedactor.RedactHeaders(h)
}

// IsSecretField checks if a field name indicates a secret.
func IsSecretField(name string) bool {
	return defaultRedactor.IsSecretField(name)
}
