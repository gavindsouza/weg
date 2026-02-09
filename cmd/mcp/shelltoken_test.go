package mcp

import (
	"reflect"
	"testing"
)

func TestTokenizeArgs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "simple words",
			input:    "hello world",
			expected: []string{"hello", "world"},
		},
		{
			name:     "double quoted argument",
			input:    `frappe --site "my site" console`,
			expected: []string{"frappe", "--site", "my site", "console"},
		},
		{
			name:     "single quoted argument",
			input:    `frappe --site 'my site' console`,
			expected: []string{"frappe", "--site", "my site", "console"},
		},
		{
			name:     "mixed quotes",
			input:    `"hello world" 'foo bar' baz`,
			expected: []string{"hello world", "foo bar", "baz"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "only whitespace",
			input:    "   \t  \n  ",
			expected: nil,
		},
		{
			name:     "extra whitespace between args",
			input:    "  hello   world  ",
			expected: []string{"hello", "world"},
		},
		{
			name:     "escaped space",
			input:    `hello\ world foo`,
			expected: []string{"hello world", "foo"},
		},
		{
			name:     "escaped quote in double quotes",
			input:    `"hello \"world\""`,
			expected: []string{`hello "world"`},
		},
		{
			name:     "single quote preserves backslash",
			input:    `'hello\nworld'`,
			expected: []string{`hello\nworld`},
		},
		{
			name:     "realistic frappe command",
			input:    "frappe --site mysite.localhost migrate",
			expected: []string{"frappe", "--site", "mysite.localhost", "migrate"},
		},
		{
			name:     "key=value args",
			input:    "doctype=User filters='{\"enabled\": 1}'",
			expected: []string{"doctype=User", `filters={"enabled": 1}`},
		},
		{
			name:     "single word",
			input:    "migrate",
			expected: []string{"migrate"},
		},
		{
			name:     "adjacent quotes",
			input:    `"hello""world"`,
			expected: []string{"helloworld"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tokenizeArgs(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("tokenizeArgs(%q)\n  got:  %v\n  want: %v", tt.input, result, tt.expected)
			}
		})
	}
}
