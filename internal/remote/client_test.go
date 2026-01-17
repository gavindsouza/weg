/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package remote

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient(t *testing.T) {
	client := NewClient("https://test.frappe.cloud", "api-key", "api-secret")

	if client.BaseURL != "https://test.frappe.cloud" {
		t.Errorf("expected BaseURL https://test.frappe.cloud, got %s", client.BaseURL)
	}

	if client.APIKey != "api-key" {
		t.Errorf("expected APIKey api-key, got %s", client.APIKey)
	}

	if client.APISecret != "api-secret" {
		t.Errorf("expected APISecret api-secret, got %s", client.APISecret)
	}

	if client.HTTPClient == nil {
		t.Error("expected HTTPClient to be initialized")
	}
}

func TestNewClientFromConfig(t *testing.T) {
	config := NewSiteConfig("https://test.frappe.cloud", "test")
	creds := &Credentials{
		Auth: CredentialAuth{
			APIKey:    "config-key",
			APISecret: "config-secret",
		},
	}

	client := NewClientFromConfig(config, creds)

	if client.BaseURL != "https://test.frappe.cloud" {
		t.Errorf("expected BaseURL from config, got %s", client.BaseURL)
	}

	if client.APIKey != "config-key" {
		t.Errorf("expected APIKey from creds, got %s", client.APIKey)
	}
}

func TestClientPing(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/method/frappe.ping" {
			t.Errorf("expected path /api/method/frappe.ping, got %s", r.URL.Path)
		}

		// Verify authorization header
		auth := r.Header.Get("Authorization")
		if auth != "token test-key:test-secret" {
			t.Errorf("expected auth header, got %s", auth)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "pong"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", "test-secret")
	if err := client.Ping(); err != nil {
		t.Errorf("Ping failed: %v", err)
	}
}

func TestClientPingError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"exc_type": "AuthenticationError"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "bad-key", "bad-secret")
	err := client.Ping()
	if err == nil {
		t.Error("expected Ping to fail with bad credentials")
	}
}

func TestClientGetDoc(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// httptest server receives decoded path
		if r.URL.Path != "/api/resource/Custom Field/User-test_field" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"name":      "User-test_field",
				"dt":        "User",
				"fieldname": "test_field",
				"fieldtype": "Data",
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "secret")
	doc, err := client.GetDoc("Custom Field", "User-test_field")
	if err != nil {
		t.Fatalf("GetDoc failed: %v", err)
	}

	if doc["name"] != "User-test_field" {
		t.Errorf("expected name User-test_field, got %v", doc["name"])
	}

	if doc["fieldtype"] != "Data" {
		t.Errorf("expected fieldtype Data, got %v", doc["fieldtype"])
	}
}

func TestClientGetList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// httptest server receives decoded path
		if r.URL.Path != "/api/resource/Client Script" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		// Verify filters are passed
		filters := r.URL.Query().Get("filters")
		if filters == "" {
			t.Error("expected filters parameter")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"name": "Script 1", "dt": "User"},
				{"name": "Script 2", "dt": "Customer"},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "secret")
	docs, err := client.GetList("Client Script", map[string]interface{}{
		"enabled": 1,
	}, []string{"name", "dt"}, 10)
	if err != nil {
		t.Fatalf("GetList failed: %v", err)
	}

	if len(docs) != 2 {
		t.Errorf("expected 2 docs, got %d", len(docs))
	}
}

func TestClientGetAll(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		w.Header().Set("Content-Type", "application/json")

		// Simulate pagination
		if callCount == 1 {
			// First page - return full page
			docs := make([]map[string]interface{}, 100)
			for i := 0; i < 100; i++ {
				docs[i] = map[string]interface{}{"name": i}
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"data": docs})
		} else {
			// Second page - return partial (end of data)
			docs := make([]map[string]interface{}, 50)
			for i := 0; i < 50; i++ {
				docs[i] = map[string]interface{}{"name": 100 + i}
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"data": docs})
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "secret")
	docs, err := client.GetAll("DocType", nil, []string{"*"})
	if err != nil {
		t.Fatalf("GetAll failed: %v", err)
	}

	if len(docs) != 150 {
		t.Errorf("expected 150 docs from pagination, got %d", len(docs))
	}

	if callCount != 2 {
		t.Errorf("expected 2 API calls for pagination, got %d", callCount)
	}
}

func TestClientCallMethod(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/method/frappe.utils.change_log.get_versions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": map[string]interface{}{
				"frappe": map[string]interface{}{
					"version": "15.0.0",
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "secret")
	result, err := client.CallMethod("frappe.utils.change_log.get_versions", nil)
	if err != nil {
		t.Fatalf("CallMethod failed: %v", err)
	}

	msg, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}

	frappe, ok := msg["frappe"].(map[string]interface{})
	if !ok {
		t.Fatal("expected frappe in result")
	}

	if frappe["version"] != "15.0.0" {
		t.Errorf("expected version 15.0.0, got %v", frappe["version"])
	}
}

func TestClientInsertDoc(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		// Verify body
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)

		if body["fieldname"] != "new_field" {
			t.Errorf("expected fieldname new_field, got %v", body["fieldname"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"name":      "User-new_field",
				"fieldname": "new_field",
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "secret")
	doc, err := client.InsertDoc("Custom Field", map[string]interface{}{
		"dt":        "User",
		"fieldname": "new_field",
		"fieldtype": "Data",
	})
	if err != nil {
		t.Fatalf("InsertDoc failed: %v", err)
	}

	if doc["name"] != "User-new_field" {
		t.Errorf("expected name User-new_field, got %v", doc["name"])
	}
}

func TestClientUpdateDoc(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("expected PUT, got %s", r.Method)
		}

		// httptest server receives decoded path
		if r.URL.Path != "/api/resource/Custom Field/User-test_field" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"name":  "User-test_field",
				"label": "Updated Label",
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "secret")
	doc, err := client.UpdateDoc("Custom Field", "User-test_field", map[string]interface{}{
		"label": "Updated Label",
	})
	if err != nil {
		t.Fatalf("UpdateDoc failed: %v", err)
	}

	if doc["label"] != "Updated Label" {
		t.Errorf("expected label Updated Label, got %v", doc["label"])
	}
}

func TestClientDeleteDoc(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("expected DELETE, got %s", r.Method)
		}

		// httptest server receives decoded path
		if r.URL.Path != "/api/resource/Custom Field/User-test_field" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "ok"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "secret")
	err := client.DeleteDoc("Custom Field", "User-test_field")
	if err != nil {
		t.Errorf("DeleteDoc failed: %v", err)
	}
}

func TestClientGetFrappeVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": map[string]interface{}{
				"frappe": map[string]interface{}{
					"version": "15.42.0",
					"title":   "Frappe Framework",
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "secret")
	version, err := client.GetFrappeVersion()
	if err != nil {
		t.Fatalf("GetFrappeVersion failed: %v", err)
	}

	if version != "15.42.0" {
		t.Errorf("expected version 15.42.0, got %s", version)
	}
}

func TestClientGetInstalledApps(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": []map[string]interface{}{
				{"app_name": "frappe", "app_version": "15.0.0", "git_branch": "version-15"},
				{"app_name": "erpnext", "app_version": "14.0.0", "git_branch": "version-14"},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "secret")
	apps, err := client.GetInstalledApps()
	if err != nil {
		t.Fatalf("GetInstalledApps failed: %v", err)
	}

	if len(apps) != 2 {
		t.Errorf("expected 2 apps, got %d", len(apps))
	}

	if apps[0]["app_name"] != "frappe" {
		t.Errorf("expected first app frappe, got %v", apps[0]["app_name"])
	}
}

func TestClientGetModules(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"name": "Core", "app_name": "frappe"},
				{"name": "Custom", "app_name": "frappe"},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "secret")
	modules, err := client.GetModules()
	if err != nil {
		t.Fatalf("GetModules failed: %v", err)
	}

	if len(modules) != 2 {
		t.Errorf("expected 2 modules, got %d", len(modules))
	}
}

func TestClientAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"exc": "Internal Server Error",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "secret")
	_, err := client.GetDoc("DocType", "Test")
	if err == nil {
		t.Error("expected error for 500 response")
	}
}
