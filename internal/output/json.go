package output

import (
	"encoding/json"
	"io"
)

// JSON prints a value as indented JSON to the configured Writer.
// Uses 2-space indentation for readability.
func JSON(v any) error {
	return JSONTo(Writer, v)
}

// JSONTo prints a value as indented JSON to a custom writer.
func JSONTo(w io.Writer, v any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}

// JSONCompact prints a value as compact JSON (no indentation).
func JSONCompact(v any) error {
	return JSONCompactTo(Writer, v)
}

// JSONCompactTo prints a value as compact JSON to a custom writer.
func JSONCompactTo(w io.Writer, v any) error {
	encoder := json.NewEncoder(w)
	return encoder.Encode(v)
}
