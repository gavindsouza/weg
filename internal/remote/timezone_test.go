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

// SiteLocation reads System Settings, then the time_zone default (which any
// user may read) when System Settings is refused, and returns UTC with an
// error only when neither answers.
func TestSiteLocation(t *testing.T) {
	const single, deflt = "/api/method/frappe.client.get_single_value", "/api/method/frappe.client.get_time_zone"
	type reply struct {
		status int
		body   string
	}
	tests := []struct {
		name    string
		replies map[string]reply
		want    string
		wantErr bool
	}{
		{"zone set", map[string]reply{single: {200, `{"message":"Europe/Berlin"}`}}, "Europe/Berlin", false},
		{"unset", map[string]reply{single: {200, `{"message":null}`}}, "Asia/Kolkata", false},
		{"no permission, default readable", map[string]reply{
			single: {403, `{"exc_type":"PermissionError"}`},
			deflt:  {200, `{"message":{"time_zone":"America/New_York"}}`}}, "America/New_York", false},
		{"transient failure, default readable", map[string]reply{
			single: {502, `bad gateway`},
			deflt:  {200, `{"message":{"time_zone":"Europe/Berlin"}}`}}, "Europe/Berlin", false},
		{"no permission, default unset", map[string]reply{
			single: {403, `{"exc_type":"PermissionError"}`},
			deflt:  {200, `{"message":{"time_zone":null}}`}}, "Asia/Kolkata", false},
		{"neither readable", map[string]reply{
			single: {403, `{"exc_type":"PermissionError"}`},
			deflt:  {403, `{"exc_type":"PermissionError"}`}}, "UTC", true},
		{"unknown zone", map[string]reply{single: {200, `{"message":"Mars/Olympus_Mons"}`}}, "UTC", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				rep, ok := tt.replies[r.URL.Path]
				if !ok {
					t.Errorf("unexpected request %s", r.URL.Path)
					rep = reply{404, `{}`}
				}
				w.WriteHeader(rep.status)
				w.Write([]byte(rep.body))
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
