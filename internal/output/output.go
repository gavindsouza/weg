// Package output provides unified output formatting for the weg CLI.
//
// It supports multiple output formats (JSON, table, plain text) controlled
// by the --output flag, and verbosity levels controlled by -v flags or
// --log-level.
//
// Commands produce structured data, and this package handles rendering
// based on user preferences and terminal capabilities.
package output

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

// Format represents the output format type.
type Format string

const (
	FormatAuto  Format = "auto"  // Detect: JSON if piped, table if TTY
	FormatJSON  Format = "json"  // JSON output (for scripting)
	FormatTable Format = "table" // Tabular output (for humans)
	FormatPlain Format = "plain" // Simple text output
	FormatQuiet Format = "quiet" // Minimal output (IDs only)
)

// Verbosity represents the verbosity level.
type Verbosity int

const (
	VerbosityQuiet   Verbosity = -1 // Errors only
	VerbosityNormal  Verbosity = 0  // Default output
	VerbosityVerbose Verbosity = 1  // + Context, what's happening
	VerbosityDebug   Verbosity = 2  // + Internal state, timing
	VerbosityTrace   Verbosity = 3  // + Network, file ops, everything
)

// DebugCategory represents a category of debug output.
type DebugCategory string

const (
	DebugAll    DebugCategory = "all"    // All categories
	DebugConfig DebugCategory = "config" // Config loading/parsing
	DebugState  DebugCategory = "state"  // State file operations
	DebugNet    DebugCategory = "net"    // HTTP/API calls
	DebugGit    DebugCategory = "git"    // Git operations
	DebugFS     DebugCategory = "fs"     // File system operations
	DebugExec   DebugCategory = "exec"   // Command execution
)

// Global configuration - set by cmd/root.go
var (
	// CurrentFormat is the active output format (default: auto)
	CurrentFormat Format = FormatAuto

	// Level is the current verbosity level
	Level Verbosity = VerbosityNormal

	// DebugCategories is the set of enabled debug categories.
	// Empty means all categories when Level >= VerbosityDebug.
	DebugCategories map[DebugCategory]bool

	// NoColor disables colored output
	NoColor bool

	// ShowTimestamps includes timestamps in debug output
	ShowTimestamps bool = true

	// Writer is the output destination (default: os.Stdout)
	Writer io.Writer = os.Stdout

	// ErrWriter is the error/debug destination (default: os.Stderr)
	ErrWriter io.Writer = os.Stderr
)

// ParseFormat converts a string to Format, returns error if invalid.
func ParseFormat(s string) (Format, error) {
	switch s {
	case "auto", "":
		return FormatAuto, nil
	case "json":
		return FormatJSON, nil
	case "table":
		return FormatTable, nil
	case "plain":
		return FormatPlain, nil
	case "quiet":
		return FormatQuiet, nil
	default:
		return FormatAuto, fmt.Errorf("invalid output format %q: must be auto, json, table, plain, or quiet", s)
	}
}

// ParseVerbosity converts a string to Verbosity level.
func ParseVerbosity(s string) (Verbosity, error) {
	switch s {
	case "quiet":
		return VerbosityQuiet, nil
	case "normal", "":
		return VerbosityNormal, nil
	case "verbose":
		return VerbosityVerbose, nil
	case "debug":
		return VerbosityDebug, nil
	case "trace":
		return VerbosityTrace, nil
	default:
		return VerbosityNormal, fmt.Errorf("invalid log level %q: must be quiet, normal, verbose, debug, or trace", s)
	}
}

// ParseDebugCategories parses a comma-separated list of debug categories.
func ParseDebugCategories(s string) {
	if DebugCategories == nil {
		DebugCategories = make(map[DebugCategory]bool)
	}

	// Simple split - avoid importing strings for this
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			if i > start {
				cat := DebugCategory(s[start:i])
				DebugCategories[cat] = true
			}
			start = i + 1
		}
	}
}

// LoadFromEnv reads settings from environment variables.
// This should be called after flag parsing, as flags take precedence.
func LoadFromEnv() {
	// Only apply env var if no explicit level was set
	if Level == VerbosityNormal {
		if level := os.Getenv("WEG_LOG_LEVEL"); level != "" {
			if v, err := ParseVerbosity(level); err == nil {
				Level = v
			}
		}
	}

	// Debug categories from env
	if cats := os.Getenv("WEG_DEBUG"); cats != "" && len(DebugCategories) == 0 {
		ParseDebugCategories(cats)
	}

	// Color settings
	if os.Getenv("WEG_NO_COLOR") != "" || os.Getenv("NO_COLOR") != "" {
		NoColor = true
	}
}

// IsTTY returns true if Writer is a terminal.
func IsTTY() bool {
	if f, ok := Writer.(*os.File); ok {
		return term.IsTerminal(int(f.Fd()))
	}
	return false
}

// EffectiveFormat returns the actual format to use,
// resolving "auto" based on terminal detection.
func EffectiveFormat() Format {
	if CurrentFormat != FormatAuto {
		return CurrentFormat
	}
	if IsTTY() {
		return FormatTable
	}
	return FormatJSON
}

// Print prints a message unless Quiet mode is enabled.
func Print(message string) {
	if Level >= VerbosityNormal {
		fmt.Fprintln(Writer, message)
	}
}

// Printf prints a formatted message unless Quiet mode is enabled.
func Printf(format string, args ...any) {
	if Level >= VerbosityNormal {
		fmt.Fprintf(Writer, format+"\n", args...)
	}
}

// Verbose prints a message only at Verbose level or higher.
func Verbose(message string) {
	if Level >= VerbosityVerbose {
		fmt.Fprintln(Writer, message)
	}
}

// Verbosef prints a formatted message only at Verbose level or higher.
func Verbosef(format string, args ...any) {
	if Level >= VerbosityVerbose {
		fmt.Fprintf(Writer, format+"\n", args...)
	}
}
