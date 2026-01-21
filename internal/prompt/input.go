package prompt

import (
	"fmt"
)

// Input prompts for text input.
//
// Example:
//
//	name, err := prompt.Input("Enter site name: ")
//	if err != nil {
//	    return err
//	}
func Input(promptText string) (string, error) {
	write("%s", promptText)

	value, err := readLine()
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}
	return value, nil
}

// InputWithDefault prompts for text input with a default value.
// If the user enters nothing, the default is returned.
//
// Example:
//
//	host, err := prompt.InputWithDefault("localhost", "Host [localhost]: ")
func InputWithDefault(defaultVal, promptText string) (string, error) {
	value, err := Input(promptText)
	if err != nil {
		return "", err
	}

	if value == "" {
		return defaultVal, nil
	}
	return value, nil
}

// InputRequired prompts for required text input.
// Keeps prompting until non-empty input is provided.
//
// Example:
//
//	name, err := prompt.InputRequired("Project name: ")
func InputRequired(promptText string) (string, error) {
	for {
		value, err := Input(promptText)
		if err != nil {
			return "", err
		}

		if value != "" {
			return value, nil
		}

		writeln("This field is required.")
	}
}
