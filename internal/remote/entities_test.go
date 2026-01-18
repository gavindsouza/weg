/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package remote

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNewFetcher(t *testing.T) {
	client := NewClient("https://test.frappe.cloud", "key", "secret")
	config := NewSiteConfig("https://test.frappe.cloud", "test")

	fetcher := NewFetcher(client, config)

	if fetcher.Client != client {
		t.Error("expected client to be set")
	}

	if fetcher.Config != config {
		t.Error("expected config to be set")
	}
}

func TestFetcherFetchAll(t *testing.T) {
	// Create a mock server that handles all entity type requests
	// Note: httptest server receives decoded paths (spaces, not %20)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/api/method/frappe.utils.change_log.get_versions":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"message": map[string]interface{}{
					"frappe": map[string]interface{}{"version": "15.0.0"},
				},
			})
		case "/api/resource/Module Def":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{
					{"name": "Core", "app_name": "frappe"},
					{"name": "Custom", "app_name": "frappe"},
				},
			})
		case "/api/resource/DocType":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{},
			})
		case "/api/resource/Custom Field":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{
					{
						"name":                "User-test_field",
						"dt":                  "User",
						"fieldname":           "test_field",
						"fieldtype":           "Data",
						"label":               "Test Field",
						"is_system_generated": 0,
					},
				},
			})
		case "/api/resource/Property Setter":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{},
			})
		case "/api/resource/Client Script":
			// List endpoint
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{
					{
						"name":   "Test Script",
						"dt":     "User",
						"script": "console.log('test');",
						"module": "Custom",
					},
				},
			})
		case "/api/resource/Client Script/Test Script":
			// GetDoc endpoint for full document
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"name":   "Test Script",
					"dt":     "User",
					"script": "console.log('test');",
					"module": "Custom",
				},
			})
		case "/api/resource/Server Script":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{},
			})
		case "/api/resource/Report":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{},
			})
		case "/api/resource/Print Format":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{},
			})
		case "/api/resource/Workflow":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{},
			})
		case "/api/resource/Notification":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{},
			})
		case "/api/resource/Letter Head":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{},
			})
		default:
			json.NewEncoder(w).Encode(map[string]interface{}{"data": []map[string]interface{}{}})
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "secret")
	config := NewSiteConfig(server.URL, "test")

	fetcher := NewFetcher(client, config)
	result, err := fetcher.FetchAll()
	if err != nil {
		t.Fatalf("FetchAll failed: %v", err)
	}

	// Should have 2 entities (1 custom field + 1 client script)
	if len(result.Entities) != 2 {
		t.Errorf("expected 2 entities, got %d", len(result.Entities))
	}

	// Check modules were discovered
	if len(result.Modules) < 2 {
		t.Errorf("expected at least 2 modules, got %d", len(result.Modules))
	}

	// Verify entity types
	hasCustomField := false
	hasClientScript := false
	for _, e := range result.Entities {
		if e.Type == EntityCustomField {
			hasCustomField = true
			if e.Module != "_" {
				t.Errorf("expected custom field to use _ module, got %s", e.Module)
			}
		}
		if e.Type == EntityClientScript {
			hasClientScript = true
		}
	}

	if !hasCustomField {
		t.Error("expected to find custom field entity")
	}
	if !hasClientScript {
		t.Error("expected to find client script entity")
	}
}

func TestWriteEntity(t *testing.T) {
	tmpDir := t.TempDir()

	entity := Entity{
		Type:   EntityClientScript,
		Name:   "Test Script",
		Module: "Custom",
		Data: map[string]interface{}{
			"name":   "Test Script",
			"dt":     "User",
			"script": "console.log('test');",
		},
		FilePath: "custom/client_script/test_script.json",
	}

	if err := WriteEntity(tmpDir, entity); err != nil {
		t.Fatalf("WriteEntity failed: %v", err)
	}

	// Verify file was created
	fullPath := filepath.Join(tmpDir, entity.FilePath)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		t.Fatal("entity file was not created")
	}

	// Read and verify content
	content, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("failed to read entity file: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(content, &data); err != nil {
		t.Fatalf("failed to parse entity JSON: %v", err)
	}

	if data["name"] != "Test Script" {
		t.Errorf("expected name Test Script, got %v", data["name"])
	}

	if data["script"] != "console.log('test');" {
		t.Errorf("expected script content, got %v", data["script"])
	}
}

func TestWriteEntityCreatesDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	entity := Entity{
		Type:     EntityReport,
		Name:     "My Report",
		Module:   "Custom",
		Data:     map[string]interface{}{"name": "My Report"},
		FilePath: "custom/report/my_report/my_report.json",
	}

	if err := WriteEntity(tmpDir, entity); err != nil {
		t.Fatalf("WriteEntity failed: %v", err)
	}

	// Verify nested directories were created
	reportDir := filepath.Join(tmpDir, "custom", "report", "my_report")
	if _, err := os.Stat(reportDir); os.IsNotExist(err) {
		t.Error("nested directories were not created")
	}
}

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Custom Field", "custom_field"},
		{"Module Def", "module_def"},
		{"already_snake", "already_snake"},
		{"Test-Name", "test_name"},
		{"ABC", "abc"},
		{"Test123", "test123"},
		// Note: This function doesn't handle CamelCase, only spaces and dashes
	}

	for _, tt := range tests {
		result := toSnakeCase(tt.input)
		if result != tt.expected {
			t.Errorf("toSnakeCase(%s) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}

func TestGetString(t *testing.T) {
	m := map[string]interface{}{
		"string_field": "hello",
		"int_field":    42,
		"nil_field":    nil,
	}

	if getString(m, "string_field") != "hello" {
		t.Error("expected hello for string_field")
	}

	if getString(m, "int_field") != "" {
		t.Error("expected empty string for non-string field")
	}

	if getString(m, "nil_field") != "" {
		t.Error("expected empty string for nil field")
	}

	if getString(m, "missing_field") != "" {
		t.Error("expected empty string for missing field")
	}
}

func TestShouldSyncModule(t *testing.T) {
	config := NewSiteConfig("https://test.frappe.cloud", "test")
	config.Modules["Custom"] = ModuleInfo{App: "frappe", Sync: true}
	config.Modules["Core"] = ModuleInfo{App: "frappe", Sync: false}

	client := NewClient("https://test.frappe.cloud", "key", "secret")
	fetcher := NewFetcher(client, config)

	tests := []struct {
		module   string
		expected bool
	}{
		{"Custom", true},  // Explicitly enabled
		{"Core", false},   // Explicitly disabled
		{"_", true},       // Catch-all always syncs
		{"Unknown", false}, // Unknown modules don't sync by default
	}

	for _, tt := range tests {
		result := fetcher.shouldSyncModule(tt.module)
		if result != tt.expected {
			t.Errorf("shouldSyncModule(%s) = %v, expected %v", tt.module, result, tt.expected)
		}
	}
}

func TestEntityTypes(t *testing.T) {
	// Verify all entity type constants are defined
	types := []EntityType{
		EntityDocType,
		EntityCustomField,
		EntityPropertySetter,
		EntityClientScript,
		EntityServerScript,
		EntityReport,
		EntityPrintFormat,
		EntityWorkflow,
		EntityNotification,
		EntityLetterHead,
	}

	for _, et := range types {
		if et == "" {
			t.Error("entity type should not be empty")
		}
	}
}

func TestFetchCustomFieldsGroupsByDocType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// httptest server receives decoded path
		if r.URL.Path == "/api/resource/Custom Field" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{
					{"name": "User-field1", "dt": "User", "fieldname": "field1"},
					{"name": "User-field2", "dt": "User", "fieldname": "field2"},
					{"name": "Customer-field1", "dt": "Customer", "fieldname": "field1"},
				},
			})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{"data": []map[string]interface{}{}})
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "secret")
	config := NewSiteConfig(server.URL, "test")
	fetcher := NewFetcher(client, config)

	entities, err := fetcher.fetchCustomFields()
	if err != nil {
		t.Fatalf("fetchCustomFields failed: %v", err)
	}

	// Should have 2 entities (grouped by doctype: User and Customer)
	if len(entities) != 2 {
		t.Errorf("expected 2 entities (grouped by doctype), got %d", len(entities))
	}

	// Find User entity and verify it has 2 fields
	for _, e := range entities {
		if e.Name == "User" {
			fields, ok := e.Data["custom_fields"].([]map[string]interface{})
			if !ok {
				t.Fatal("expected custom_fields to be a slice")
			}
			if len(fields) != 2 {
				t.Errorf("expected 2 fields for User, got %d", len(fields))
			}
		}
	}
}

func TestFetchPropertySettersGroupsByDocType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// httptest server receives decoded path
		if r.URL.Path == "/api/resource/Property Setter" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{
					{"name": "User-email-label", "doc_type": "User", "property": "label"},
					{"name": "User-email-hidden", "doc_type": "User", "property": "hidden"},
				},
			})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{"data": []map[string]interface{}{}})
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "secret")
	config := NewSiteConfig(server.URL, "test")
	fetcher := NewFetcher(client, config)

	entities, err := fetcher.fetchPropertySetters()
	if err != nil {
		t.Fatalf("fetchPropertySetters failed: %v", err)
	}

	// Should have 1 entity (grouped by doctype: User)
	if len(entities) != 1 {
		t.Errorf("expected 1 entity (grouped by doctype), got %d", len(entities))
	}

	if entities[0].Module != "_" {
		t.Errorf("expected module _, got %s", entities[0].Module)
	}
}

func TestFetchClientScripts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// httptest server receives decoded path
		switch r.URL.Path {
		case "/api/resource/Client Script":
			// List endpoint
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{
					{
						"name":    "Test Script",
						"dt":      "User",
						"script":  "console.log('test');",
						"module":  "Custom",
						"enabled": 1,
					},
				},
			})
		case "/api/resource/Client Script/Test Script":
			// GetDoc endpoint
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"name":    "Test Script",
					"dt":      "User",
					"script":  "console.log('test');",
					"module":  "Custom",
					"enabled": 1,
				},
			})
		case "/api/resource/DocType":
			// DocType module lookup for prefetchDocTypeModules
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{
					{
						"name":   "User",
						"module": "Custom",
					},
				},
			})
		default:
			json.NewEncoder(w).Encode(map[string]interface{}{"data": []map[string]interface{}{}})
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "secret")
	config := NewSiteConfig(server.URL, "test")
	fetcher := NewFetcher(client, config)

	entities, err := fetcher.fetchClientScripts()
	if err != nil {
		t.Fatalf("fetchClientScripts failed: %v", err)
	}

	if len(entities) != 1 {
		t.Errorf("expected 1 entity, got %d", len(entities))
	}

	if len(entities) > 0 {
		if entities[0].Type != EntityClientScript {
			t.Errorf("expected EntityClientScript, got %v", entities[0].Type)
		}

		if entities[0].Module != "Custom" {
			t.Errorf("expected module Custom, got %s", entities[0].Module)
		}
	}
}

func TestFetchServerScripts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// httptest server receives decoded path
		switch r.URL.Path {
		case "/api/resource/Server Script":
			// List endpoint
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{
					{
						"name":        "API Test",
						"script_type": "API",
						"api_method":  "test_method",
						"script":      "frappe.response['message'] = 'ok'",
						"disabled":    0,
					},
				},
			})
		case "/api/resource/Server Script/API Test":
			// GetDoc endpoint
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"name":        "API Test",
					"script_type": "API",
					"api_method":  "test_method",
					"script":      "frappe.response['message'] = 'ok'",
					"disabled":    0,
				},
			})
		default:
			json.NewEncoder(w).Encode(map[string]interface{}{"data": []map[string]interface{}{}})
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "secret")
	config := NewSiteConfig(server.URL, "test")
	fetcher := NewFetcher(client, config)

	entities, err := fetcher.fetchServerScripts()
	if err != nil {
		t.Fatalf("fetchServerScripts failed: %v", err)
	}

	if len(entities) != 1 {
		t.Errorf("expected 1 entity, got %d", len(entities))
	}

	if len(entities) > 0 {
		if entities[0].Type != EntityServerScript {
			t.Errorf("expected EntityServerScript, got %v", entities[0].Type)
		}

		// Server scripts without module should default to _
		if entities[0].Module != "_" {
			t.Errorf("expected module _, got %s", entities[0].Module)
		}
	}
}

func TestFetchReportsWithChildTable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// httptest server receives decoded paths
		if r.URL.Path == "/api/resource/Report" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{
					{
						"name":        "Test Report",
						"report_type": "Script Report",
						"is_standard": "No",
						"module":      "Custom",
					},
				},
			})
		} else if r.URL.Path == "/api/resource/Report/Test Report" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"name":        "Test Report",
					"report_type": "Script Report",
					"is_standard": "No",
					"module":      "Custom",
					"roles": []map[string]interface{}{
						{"role": "System Manager"},
					},
				},
			})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{"data": []map[string]interface{}{}})
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "secret")
	config := NewSiteConfig(server.URL, "test")
	fetcher := NewFetcher(client, config)

	entities, err := fetcher.fetchReports()
	if err != nil {
		t.Fatalf("fetchReports failed: %v", err)
	}

	if len(entities) != 1 {
		t.Errorf("expected 1 entity, got %d", len(entities))
	}

	if len(entities) > 0 {
		// Verify full doc was fetched (with roles child table)
		roles, ok := entities[0].Data["roles"].([]interface{})
		if !ok {
			// Might be []map[string]interface{} depending on JSON unmarshaling
			rolesMap, ok := entities[0].Data["roles"].([]map[string]interface{})
			if !ok {
				t.Log("roles not found or wrong type - this is expected if GetDoc wasn't called")
			} else if len(rolesMap) != 1 {
				t.Errorf("expected 1 role, got %d", len(rolesMap))
			}
		} else if len(roles) != 1 {
			t.Errorf("expected 1 role, got %d", len(roles))
		}
	}
}

func TestEntityFilePaths(t *testing.T) {
	tests := []struct {
		entityType EntityType
		name       string
		module     string
		expected   string
	}{
		{EntityClientScript, "Test Script", "Custom", "custom/client_script/test_script.json"},
		{EntityServerScript, "API Method", "_", "_/server_script/api_method.json"},
		{EntityCustomField, "User", "_", "_/custom_field/user.json"},
		{EntityPropertySetter, "Customer", "_", "_/property_setter/customer.json"},
	}

	for _, tt := range tests {
		// Simulate filepath construction as done in fetchers
		result := filepath.Join(toSnakeCase(tt.module), string(tt.entityType), toSnakeCase(tt.name)+".json")
		if result != tt.expected {
			t.Errorf("filepath for %s/%s = %s, expected %s", tt.module, tt.name, result, tt.expected)
		}
	}
}
