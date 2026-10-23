/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package remote

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPathSegment(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Sales Invoice", "sales_invoice"},
		{"my-script", "my_script"},
		{"Sales/Purchase", "sales_purchase"},
		{`a\b`, "a_b"},
		{"..", "__"},
		{"../../etc/passwd", "___.._etc_passwd"},
		{".git", "_git"},
		{".hidden.name", "_hidden.name"},
		{"v1.2", "v1.2"},
		{"", "_"},
	}
	for _, tt := range tests {
		if got := pathSegment(tt.in); got != tt.want {
			t.Errorf("pathSegment(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// A site-controlled name or module cannot place a file outside its
// <module>/<type>/ directory.
func TestBuildFilePathContainsNames(t *testing.T) {
	tests := []struct {
		et           EntityType
		name, module string
		want         string
	}{
		{EntityServerScript, "Calc", "Projects", "projects/server_script/calc.json"},
		{EntityDocType, "Gift Card", "Custom", "custom/doctype/gift_card/gift_card.json"},
		{EntityServerScript, "../../../victim", "Projects", "projects/server_script/___.._.._victim.json"},
		{EntityReport, "..", "Projects", "projects/report/__/__.json"},
		{EntityPrintFormat, "Invoice", "../..", "___../print_format/invoice.json"},
		{EntityClientScript, "x", ".git", "_git/client_script/x.json"},
		{EntityPrintFormat, "Sales/Purchase", "Accounts", "accounts/print_format/sales_purchase.json"},
	}
	for _, tt := range tests {
		got := buildFilePath(tt.et, tt.name, tt.module)
		if got != tt.want {
			t.Errorf("buildFilePath(%s, %q, %q) = %q, want %q", tt.et, tt.name, tt.module, got, tt.want)
		}
		if err := CheckExportPath(got); err != nil {
			t.Errorf("buildFilePath(%s, %q, %q) = %q is refused: %v", tt.et, tt.name, tt.module, got, err)
		}
		if _, ok := entityTypeOfPath(got); !ok {
			t.Errorf("buildFilePath(%s, %q, %q) = %q is not recognised as an export file", tt.et, tt.name, tt.module, got)
		}
	}
}

func TestCheckExportPath(t *testing.T) {
	ok := []string{"projects/server_script/calc.json", "_/doctype/x/x.json", "modules.txt"}
	bad := []string{"", "../x.json", "a/../../x.json", "a/../b.json", "/etc/x.json", "./a.json",
		".git/hooks/x.json", ".weg/site.toml", "a//b.json"}
	for _, p := range ok {
		if err := CheckExportPath(p); err != nil {
			t.Errorf("CheckExportPath(%q) = %v, want nil", p, err)
		}
	}
	for _, p := range bad {
		if err := CheckExportPath(p); err == nil {
			t.Errorf("CheckExportPath(%q) accepted", p)
		}
	}
}

func TestWriteEntityRefusesEscapingPath(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "catalog")
	err := WriteEntity(base, Entity{Name: "../victim", FilePath: "../victim.json", Data: map[string]any{"a": 1}})
	if err == nil || !strings.Contains(err.Error(), "refusing export path") {
		t.Fatalf("WriteEntity error = %v, want a refusal", err)
	}
	if entries, _ := os.ReadDir(root); len(entries) != 0 {
		t.Errorf("wrote %v", entries)
	}
}

func TestCheckEntityPaths(t *testing.T) {
	entities := []Entity{
		{Type: EntityServerScript, Name: "Calc", FilePath: "projects/server_script/calc.json"},
		{Type: EntityServerScript, Name: "../victim", FilePath: "../victim.json"},
	}
	err := CheckEntityPaths(entities)
	if err == nil || !strings.Contains(err.Error(), `"../victim"`) || strings.Contains(err.Error(), "Calc") {
		t.Errorf("CheckEntityPaths = %v, want only ../victim named", err)
	}
	if err := CheckEntityPaths(entities[:1]); err != nil {
		t.Errorf("CheckEntityPaths(safe) = %v", err)
	}
}
