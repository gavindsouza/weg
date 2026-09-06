package cmd

import "testing"

func TestWegDevFrappe(t *testing.T) {
	content := `[project]
name = "demo"

[tool.weg]
compatibility.frappe = ["13","14","15","16","develop"]

[tool.weg.dev]
frappe = "15"
database = "mariadb"
`
	got := wegDevFrappe(content)
	if got != "15" {
		t.Fatalf("wegDevFrappe = %q, want %q", got, "15")
	}

	section := `[tool.weg]
version = "demo-1.0.0"
compatibility.frappe = ["13","14","15","16","develop"]

[tool.weg.dev]
frappe = "13"
database = "mariadb"
`
	patched := setWegDevFrappe(section, "15")
	if got := wegDevFrappe(patched); got != "15" {
		t.Fatalf("setWegDevFrappe left dev at %q", got)
	}
	if got := wegDevFrappe("[tool.weg]\nfoo = \"bar\""); got != "" {
		t.Fatalf("wegDevFrappe on section without dev table = %q", got)
	}
}