package remote

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolveLocation(t *testing.T) {
	tests := []struct {
		name    string
		want    string
		wantErr bool
	}{
		{"Europe/Berlin", "Europe/Berlin", false},
		{"Asia/Kolkata", "Asia/Kolkata", false},
		{"", "Asia/Kolkata", false}, // Frappe's default when unset
		{"Mars/Olympus_Mons", "UTC", true},
	}
	for _, tt := range tests {
		loc, err := resolveLocation(tt.name)
		if loc.String() != tt.want || (err != nil) != tt.wantErr {
			t.Errorf("resolveLocation(%q) = %s, %v; want %s, err=%v", tt.name, loc, err, tt.want, tt.wantErr)
		}
	}
}

// SiteLocation reads System Settings and falls back to UTC (with an error
// for the caller to warn about) when the site does not answer.
func TestSiteLocation(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		want    string
		wantErr bool
	}{
		{"zone set", 200, `{"message":"Europe/Berlin"}`, "Europe/Berlin", false},
		{"no permission", 403, `{"exc_type":"PermissionError"}`, "UTC", true},
		{"unset", 200, `{"message":null}`, "Asia/Kolkata", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/method/frappe.client.get_single_value" {
					t.Errorf("unexpected request %s", r.URL.Path)
				}
				w.WriteHeader(tt.status)
				w.Write([]byte(tt.body))
			}))
			defer srv.Close()
			f := &Fetcher{Client: NewClient(srv.URL, "k", "s")}
			loc, err := f.SiteLocation()
			if loc.String() != tt.want || (err != nil) != tt.wantErr {
				t.Fatalf("SiteLocation() = %s, %v; want %s, err=%v", loc, err, tt.want, tt.wantErr)
			}
		})
	}
}
