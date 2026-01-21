package output

import (
	"fmt"
)

// Unicode symbols for status indicators
const (
	SymbolSuccess = "✓"
	SymbolError   = "✗"
	SymbolWarning = "⚠"
	SymbolInfo    = "→"
	SymbolPending = "○"
	SymbolActive  = "●"
)

// Status prints a status line with a symbol prefix.
// In JSON mode, this is a no-op (status goes in result).
//
// Example: Status(SymbolSuccess, "Connected") prints "✓ Connected"
func Status(symbol, message string) {
	if Level < VerbosityNormal {
		return
	}
	if EffectiveFormat() == FormatJSON {
		return // Status messages don't go to JSON output
	}
	fmt.Fprintf(Writer, "%s %s\n", symbol, message)
}

// Statusf prints a formatted status line with a symbol prefix.
func Statusf(symbol, format string, args ...any) {
	if Level < VerbosityNormal {
		return
	}
	if EffectiveFormat() == FormatJSON {
		return
	}
	fmt.Fprintf(Writer, "%s %s\n", symbol, fmt.Sprintf(format, args...))
}

// Success prints a success message with checkmark.
// Example: Success("Connected") prints "✓ Connected"
func Success(message string) {
	Status(SymbolSuccess, message)
}

// Successf prints a formatted success message with checkmark.
func Successf(format string, args ...any) {
	Statusf(SymbolSuccess, format, args...)
}

// Error prints an error message to stderr with X mark.
// This always prints regardless of verbosity level.
// Example: Error("Connection failed") prints "✗ Connection failed"
func Error(message string) {
	fmt.Fprintf(ErrWriter, "%s %s\n", SymbolError, message)
}

// Errorf prints a formatted error message to stderr.
func Errorf(format string, args ...any) {
	fmt.Fprintf(ErrWriter, "%s %s\n", SymbolError, fmt.Sprintf(format, args...))
}

// Warning prints a warning message to stderr with warning symbol.
// This prints at Normal level and above.
// Example: Warning("Config not found, using defaults") prints "⚠ Config not found..."
func Warning(message string) {
	if Level < VerbosityNormal {
		return
	}
	fmt.Fprintf(ErrWriter, "%s %s\n", SymbolWarning, message)
}

// Warningf prints a formatted warning message to stderr.
func Warningf(format string, args ...any) {
	if Level < VerbosityNormal {
		return
	}
	fmt.Fprintf(ErrWriter, "%s %s\n", SymbolWarning, fmt.Sprintf(format, args...))
}

// Info prints an info message with arrow symbol.
// Example: Info("Using config from ~/.weg") prints "→ Using config from ~/.weg"
func Info(message string) {
	Status(SymbolInfo, message)
}

// Infof prints a formatted info message with arrow symbol.
func Infof(format string, args ...any) {
	Statusf(SymbolInfo, format, args...)
}

// Step prints a numbered step indicator.
// Example: Step(1, 3, "Loading config") prints "[1/3] Loading config"
func Step(current, total int, message string) {
	if Level < VerbosityNormal {
		return
	}
	if EffectiveFormat() == FormatJSON {
		return
	}
	fmt.Fprintf(Writer, "[%d/%d] %s\n", current, total, message)
}

// Stepf prints a formatted numbered step indicator.
func Stepf(current, total int, format string, args ...any) {
	if Level < VerbosityNormal {
		return
	}
	if EffectiveFormat() == FormatJSON {
		return
	}
	fmt.Fprintf(Writer, "[%d/%d] %s\n", current, total, fmt.Sprintf(format, args...))
}
