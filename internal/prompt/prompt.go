// Package prompt provides user interaction utilities for the weg CLI.
//
// It centralizes confirmation dialogs, password input, and string prompts
// with consistent behavior across all commands.
//
// The package respects:
// - AssumeYes: Auto-confirm when --yes/-y is passed
// - Terminal detection: Appropriate behavior for TTY vs piped input
// - Output format: Skips prompts in JSON/quiet mode
package prompt

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// Config holds prompt configuration.
// These are set by cmd/root.go during initialization.
var (
	// AssumeYes skips confirmations when true (--yes/-y flag)
	AssumeYes bool

	// Reader is the input source (default: os.Stdin)
	Reader io.Reader = os.Stdin

	// Writer is the output destination for prompts (default: os.Stderr)
	// Prompts go to stderr so stdout remains clean for data
	Writer io.Writer = os.Stderr

	// bufReader caches the buffered reader for the current Reader
	bufReader *bufio.Reader
)

// IsTTY returns true if stdin is a terminal.
func IsTTY() bool {
	if f, ok := Reader.(*os.File); ok {
		return term.IsTerminal(int(f.Fd()))
	}
	return false
}

// getReader returns a buffered reader for the configured Reader.
// Caches the bufio.Reader to handle multiple reads correctly.
func getReader() *bufio.Reader {
	if bufReader == nil {
		bufReader = bufio.NewReader(Reader)
	}
	return bufReader
}

// ResetReader clears the cached reader.
// Call this when changing the Reader (mainly for testing).
func ResetReader() {
	bufReader = nil
}

// readLine reads a line from the configured Reader.
func readLine() (string, error) {
	reader := getReader()
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// write writes to the configured Writer.
func write(format string, args ...any) {
	fmt.Fprintf(Writer, format, args...)
}

// writeln writes a line to the configured Writer.
func writeln(format string, args ...any) {
	fmt.Fprintf(Writer, format+"\n", args...)
}
