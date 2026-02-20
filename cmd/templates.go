package cmd

import (
	"embed"
	"strings"
)

//go:embed templates/*
var templateFS embed.FS

// tmpl reads an embedded template file and returns its content.
func tmpl(name string) string {
	data, err := templateFS.ReadFile("templates/" + name)
	if err != nil {
		panic("missing embedded template: " + name)
	}
	return string(data)
}

// tmplReplace reads an embedded template and performs placeholder substitutions.
func tmplReplace(name string, replacements map[string]string) string {
	content := tmpl(name)
	for placeholder, value := range replacements {
		content = strings.ReplaceAll(content, "{{"+placeholder+"}}", value)
	}
	return content
}
