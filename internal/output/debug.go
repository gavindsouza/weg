package output

import (
	"fmt"
	"time"
)

// DebugEnabled returns true if debug output is enabled for the given category.
func DebugEnabled(category DebugCategory) bool {
	if Level < VerbosityDebug {
		return false
	}

	// If no categories specified, all are enabled
	if len(DebugCategories) == 0 {
		return true
	}

	// Check if "all" is enabled
	if DebugCategories[DebugAll] {
		return true
	}

	return DebugCategories[category]
}

// TraceEnabled returns true if trace output is enabled for the given category.
func TraceEnabled(category DebugCategory) bool {
	if Level < VerbosityTrace {
		return false
	}

	// If no categories specified, all are enabled
	if len(DebugCategories) == 0 {
		return true
	}

	// Check if "all" is enabled
	if DebugCategories[DebugAll] {
		return true
	}

	return DebugCategories[category]
}

// Debug prints a debug message if the category is enabled.
// Output goes to stderr with timestamp and category prefix.
//
// Example: Debug(DebugConfig, "Loading config") prints:
// [DEBUG 14:23:01.123] config: Loading config
func Debug(category DebugCategory, message string) {
	if !DebugEnabled(category) {
		return
	}
	writeDebug("DEBUG", category, message)
}

// Debugf prints a formatted debug message if the category is enabled.
func Debugf(category DebugCategory, format string, args ...any) {
	if !DebugEnabled(category) {
		return
	}
	writeDebug("DEBUG", category, fmt.Sprintf(format, args...))
}

// Trace prints a trace message if the category is enabled.
// Trace is more verbose than debug, typically showing I/O operations.
//
// Example: Trace(DebugNet, "POST /api/method") prints:
// [TRACE 14:23:01.123] net: POST /api/method
func Trace(category DebugCategory, message string) {
	if !TraceEnabled(category) {
		return
	}
	writeDebug("TRACE", category, message)
}

// Tracef prints a formatted trace message if the category is enabled.
func Tracef(category DebugCategory, format string, args ...any) {
	if !TraceEnabled(category) {
		return
	}
	writeDebug("TRACE", category, fmt.Sprintf(format, args...))
}

// writeDebug writes a debug/trace line to stderr.
func writeDebug(level string, category DebugCategory, message string) {
	if ShowTimestamps {
		ts := time.Now().Format("15:04:05.000")
		fmt.Fprintf(ErrWriter, "[%s %s] %s: %s\n", level, ts, category, message)
	} else {
		fmt.Fprintf(ErrWriter, "[%s] %s: %s\n", level, category, message)
	}
}

// WithTiming returns a function that, when called, logs the duration of an operation.
// Use with defer for automatic timing:
//
//	defer output.WithTiming(output.DebugConfig, "parse config")()
//
// This will print: [DEBUG 14:23:01.123] config: parse config took 5ms
func WithTiming(category DebugCategory, operation string) func() {
	if !DebugEnabled(category) {
		return func() {} // No-op if debug not enabled
	}

	start := time.Now()
	return func() {
		elapsed := time.Since(start)
		Debugf(category, "%s took %s", operation, formatDuration(elapsed))
	}
}

// formatDuration formats a duration in a human-readable way.
func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
	return fmt.Sprintf("%.1fm", d.Minutes())
}

// TraceHTTPRequest logs an HTTP request at trace level.
// Headers are automatically redacted.
func TraceHTTPRequest(method, url string, headers map[string]string, body []byte) {
	if !TraceEnabled(DebugNet) {
		return
	}

	Tracef(DebugNet, "%s %s", method, url)

	if len(headers) > 0 {
		Trace(DebugNet, "Headers:")
		for k, v := range headers {
			if IsSecretField(k) {
				v = Redact(v)
			}
			Tracef(DebugNet, "  %s: %s", k, v)
		}
	}

	if len(body) > 0 {
		// Redact any secrets in the body
		redacted := RedactString(string(body))
		// Truncate if too long
		if len(redacted) > 500 {
			redacted = redacted[:500] + "..."
		}
		Tracef(DebugNet, "Body: %s", redacted)
	}
}

// TraceHTTPResponse logs an HTTP response at trace level.
func TraceHTTPResponse(statusCode int, status string, elapsed time.Duration, body []byte) {
	if !TraceEnabled(DebugNet) {
		return
	}

	Tracef(DebugNet, "Response: %d %s (%s)", statusCode, status, formatDuration(elapsed))

	if len(body) > 0 {
		// Redact any secrets in the body
		redacted := RedactString(string(body))
		// Truncate if too long
		if len(redacted) > 500 {
			redacted = redacted[:500] + "..."
		}
		Tracef(DebugNet, "Body: %s", redacted)
	}
}
