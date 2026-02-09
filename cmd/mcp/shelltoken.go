package mcp

// tokenizeArgs splits a command string into arguments, respecting
// single and double quotes. This replaces strings.Fields() which
// breaks quoted arguments like "my site" into separate tokens.
//
// Examples:
//
//	"hello world"       → ["hello", "world"]
//	"'my site' list"    → ["my site", "list"]
//	`"my site" list`    → ["my site", "list"]
//	"it's fine"         → ["it's", "fine"]
func tokenizeArgs(s string) []string {
	var args []string
	var current []byte
	var quote byte
	escaped := false

	for i := 0; i < len(s); i++ {
		c := s[i]

		if escaped {
			current = append(current, c)
			escaped = false
			continue
		}

		if c == '\\' && quote != '\'' {
			escaped = true
			continue
		}

		if quote != 0 {
			if c == quote {
				quote = 0
			} else {
				current = append(current, c)
			}
			continue
		}

		switch c {
		case '\'', '"':
			quote = c
		case ' ', '\t', '\n', '\r':
			if len(current) > 0 {
				args = append(args, string(current))
				current = current[:0]
			}
		default:
			current = append(current, c)
		}
	}

	if len(current) > 0 {
		args = append(args, string(current))
	}

	return args
}
