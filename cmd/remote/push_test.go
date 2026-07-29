/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package remote

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gavindsouza/weg/internal/remote"
)

// fakeFrappe is an httptest-backed fake of the Frappe /api/resource endpoints.
// It mimics the behaviors push depends on: 404 for missing documents, and
// (like Frappe's check_if_latest) rejecting updates whose `modified` doesn't
// match the stored document.
type fakeFrappe struct {
	t    *testing.T
	docs map[string]map[string]any // "DocType/name" -> doc

	gets    int
	puts    int
	posts   int
	deletes int

	server *httptest.Server
}

func newFakeFrappe(t *testing.T) *fakeFrappe {
	t.Helper()
	f := &fakeFrappe{t: t, docs: make(map[string]map[string]any)}
	f.server = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeFrappe) client() *remote.Client {
	return remote.NewClient(f.server.URL, "key", "secret")
}

func (f *fakeFrappe) key(path string) string {
	return strings.TrimPrefix(path, "/api/resource/")
}

func (f *fakeFrappe) handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if !strings.HasPrefix(r.URL.Path, "/api/resource/") {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	key := f.key(r.URL.Path)

	switch r.Method {
	case http.MethodGet:
		f.gets++
		doc, ok := f.docs[key]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]any{"exc_type": "DoesNotExistError"})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"data": doc})

	case http.MethodPut:
		f.puts++
		doc, ok := f.docs[key]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]any{"exc_type": "DoesNotExistError"})
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		// Frappe's check_if_latest: stale `modified` is rejected
		if bm, _ := body["modified"].(string); bm != doc["modified"] {
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]any{"exc_type": "TimestampMismatchError"})
			return
		}
		for k, v := range body {
			doc[k] = v
		}
		doc["modified"] = "2026-07-29 12:00:00.000000" // server bumps on save
		json.NewEncoder(w).Encode(map[string]any{"data": doc})

	case http.MethodPost:
		f.posts++
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		name, _ := body["name"].(string)
		body["modified"] = "2026-07-29 12:00:00.000000"
		f.docs[key+"/"+name] = body
		json.NewEncoder(w).Encode(map[string]any{"data": body})

	case http.MethodDelete:
		f.deletes++
		if _, ok := f.docs[key]; !ok {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]any{"exc_type": "DoesNotExistError"})
			return
		}
		delete(f.docs, key)
		json.NewEncoder(w).Encode(map[string]any{"message": "ok"})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func TestPushDocSkipsUnchanged(t *testing.T) {
	f := newFakeFrappe(t)
	f.docs["Server Script/Auto-Attendance"] = map[string]any{
		"name":     "Auto-Attendance",
		"doctype":  "Server Script",
		"script":   "do_it()",
		"modified": "2026-07-01 10:00:00.000000",
	}

	// Same content, older local modified: must be a no-op, not an update
	local := map[string]any{
		"name":     "Auto-Attendance",
		"doctype":  "Server Script",
		"script":   "do_it()",
		"modified": "2026-06-01 10:00:00.000000",
	}

	_, outcome, err := pushDoc(f.client(), "Server Script", "Auto-Attendance", local, false)
	if err != nil {
		t.Fatalf("pushDoc failed: %v", err)
	}
	if outcome != outcomeSkipped {
		t.Errorf("expected outcomeSkipped, got %v", outcome)
	}
	if f.puts != 0 {
		t.Errorf("expected no PUT for unchanged doc, got %d", f.puts)
	}
}

func TestPushDocUpdatesAndCopiesRemoteModified(t *testing.T) {
	f := newFakeFrappe(t)
	f.docs["Server Script/Auto-Attendance"] = map[string]any{
		"name":     "Auto-Attendance",
		"doctype":  "Server Script",
		"script":   "old()",
		"modified": "2026-07-01 10:00:00.000000",
	}

	// Local edit based on the current remote (modified matches)
	local := map[string]any{
		"name":     "Auto-Attendance",
		"doctype":  "Server Script",
		"script":   "new()",
		"modified": "2026-07-01 10:00:00.000000",
	}

	saved, outcome, err := pushDoc(f.client(), "Server Script", "Auto-Attendance", local, false)
	if err != nil {
		t.Fatalf("pushDoc failed: %v", err)
	}
	if outcome != outcomeUpdated {
		t.Errorf("expected outcomeUpdated, got %v", outcome)
	}
	if f.puts != 1 {
		t.Errorf("expected 1 PUT, got %d", f.puts)
	}
	if f.docs["Server Script/Auto-Attendance"]["script"] != "new()" {
		t.Errorf("remote script not updated: %v", f.docs["Server Script/Auto-Attendance"]["script"])
	}
	if saved["modified"] != "2026-07-29 12:00:00.000000" {
		t.Errorf("expected saved doc with server's new modified, got %v", saved["modified"])
	}
}

func TestPushDocConflictWithoutForce(t *testing.T) {
	f := newFakeFrappe(t)
	f.docs["Server Script/Auto-Attendance"] = map[string]any{
		"name":     "Auto-Attendance",
		"doctype":  "Server Script",
		"script":   "remote_edit()",
		"modified": "2026-07-20 10:00:00.000000",
	}

	// Local edit based on an older remote state: remote changed in between
	local := map[string]any{
		"name":     "Auto-Attendance",
		"doctype":  "Server Script",
		"script":   "local_edit()",
		"modified": "2026-07-01 10:00:00.000000",
	}

	_, _, err := pushDoc(f.client(), "Server Script", "Auto-Attendance", local, false)
	if err == nil {
		t.Fatal("expected conflict error, got nil")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("expected error to mention --force, got: %v", err)
	}
	if f.puts != 0 {
		t.Errorf("expected no PUT on conflict, got %d", f.puts)
	}
	if f.docs["Server Script/Auto-Attendance"]["script"] != "remote_edit()" {
		t.Errorf("remote doc must not be overwritten on conflict")
	}

	// With force, the push must go through, using the remote's modified so the
	// server's timestamp check passes
	_, outcome, err := pushDoc(f.client(), "Server Script", "Auto-Attendance", local, true)
	if err != nil {
		t.Fatalf("forced pushDoc failed: %v", err)
	}
	if outcome != outcomeUpdated {
		t.Errorf("expected outcomeUpdated, got %v", outcome)
	}
	if f.docs["Server Script/Auto-Attendance"]["script"] != "local_edit()" {
		t.Errorf("forced push did not apply local edit")
	}
}

func TestPushDocInsertsWhenMissing(t *testing.T) {
	f := newFakeFrappe(t)

	local := map[string]any{
		"name":    "New Script",
		"doctype": "Server Script",
		"script":  "new()",
	}

	_, outcome, err := pushDoc(f.client(), "Server Script", "New Script", local, false)
	if err != nil {
		t.Fatalf("pushDoc failed: %v", err)
	}
	if outcome != outcomeCreated {
		t.Errorf("expected outcomeCreated, got %v", outcome)
	}
	if f.posts != 1 {
		t.Errorf("expected 1 POST, got %d", f.posts)
	}
	if _, ok := f.docs["Server Script/New Script"]; !ok {
		t.Error("document was not created on remote")
	}
}

func TestPushDocDoesNotMaskServerErrorAsInsert(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		t.Errorf("unexpected %s request after failed GET", r.Method)
	}))
	defer server.Close()

	client := remote.NewClient(server.URL, "key", "secret")
	local := map[string]any{"name": "Doc", "doctype": "Server Script"}

	_, _, err := pushDoc(client, "Server Script", "Doc", local, false)
	if err == nil {
		t.Fatal("expected error for server failure, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected the 500 to surface, got: %v", err)
	}
}

func TestPushEntityPropertySettersUseRemoteTimestamp(t *testing.T) {
	f := newFakeFrappe(t)
	// Remote setter was touched after our last sync (modified differs)
	f.docs["Property Setter/Task-status-options"] = map[string]any{
		"name":     "Task-status-options",
		"doctype":  "Property Setter",
		"doc_type": "Task",
		"property": "options",
		"value":    "Open\nDone",
		"modified": "2026-07-20 10:00:00.000000",
	}

	dir := t.TempDir()
	filePath := filepath.Join(dir, "task.json")
	entity := localEntity{
		filePath:   filePath,
		entityType: "property_setter",
		doctype:    "Property Setter",
		name:       "Task",
		data: map[string]any{
			"doctype": "Task",
			"property_setters": []any{
				map[string]any{
					"name":     "Task-status-options",
					"doctype":  "Property Setter",
					"doc_type": "Task",
					"property": "options",
					"value":    "Open\nBlocked\nDone",
					"modified": "2026-07-01 10:00:00.000000",
				},
			},
		},
	}

	// Without force this is a conflict (remote is newer), not a stomp
	if _, err := pushEntity(f.client(), entity, false); err == nil {
		t.Fatal("expected conflict error for remotely-changed property setter")
	}
	if f.puts != 0 {
		t.Errorf("expected no PUT on conflict, got %d", f.puts)
	}

	// With force the update must pass the server's timestamp check, which
	// requires sending the remote's modified (the old code sent the stale
	// local one and got TimestampMismatchError)
	stats, err := pushEntity(f.client(), entity, true)
	if err != nil {
		t.Fatalf("forced pushEntity failed: %v", err)
	}
	if stats.updated != 1 {
		t.Errorf("expected 1 updated, got %+v", stats)
	}
	if f.docs["Property Setter/Task-status-options"]["value"] != "Open\nBlocked\nDone" {
		t.Errorf("property setter value not updated on remote")
	}
}

func TestPushDocumentWritesBackSavedDoc(t *testing.T) {
	f := newFakeFrappe(t)
	f.docs["Server Script/Auto-Attendance"] = map[string]any{
		"name":     "Auto-Attendance",
		"doctype":  "Server Script",
		"script":   "old()",
		"modified": "2026-07-01 10:00:00.000000",
	}

	dir := t.TempDir()
	filePath := filepath.Join(dir, "auto_attendance.json")
	data := map[string]any{
		"name":     "Auto-Attendance",
		"doctype":  "Server Script",
		"script":   "new()",
		"modified": "2026-07-01 10:00:00.000000",
	}
	writeLocalDoc(filePath, data)

	entity := localEntity{
		filePath:   filePath,
		entityType: "server_script",
		doctype:    "Server Script",
		name:       "Auto-Attendance",
		data:       data,
	}

	stats, err := pushDocument(f.client(), entity, false)
	if err != nil {
		t.Fatalf("pushDocument failed: %v", err)
	}
	if stats.updated != 1 {
		t.Errorf("expected 1 updated, got %+v", stats)
	}

	// The local file must now track the remote's post-save state, so the next
	// push doesn't see our own change as a remote conflict
	raw, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read written-back file: %v", err)
	}
	var written map[string]any
	if err := json.Unmarshal(raw, &written); err != nil {
		t.Fatalf("written-back file is not valid JSON: %v", err)
	}
	if written["modified"] != "2026-07-29 12:00:00.000000" {
		t.Errorf("expected written-back modified to match server, got %v", written["modified"])
	}
	if written["script"] != "new()" {
		t.Errorf("expected written-back script new(), got %v", written["script"])
	}
}

func TestTypeToDocType(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"server_script", "Server Script"},
		{"client_script", "Client Script"},
		{"custom_field", "Custom Field"},
		{"property_setter", "Property Setter"},
		{"web_template", "Web Template"},
		{"letter_head", "Letter Head"},
		{"doctype", "DocType"},
	}
	for _, tt := range tests {
		if got := typeToDocType(tt.in); got != tt.want {
			t.Errorf("typeToDocType(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// gitRun runs a git command in dir, failing the test on error.
func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	base := []string{"-c", "user.email=test@example.com", "-c", "user.name=Test"}
	cmd := exec.Command("git", append(base, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestFindDeletedEntitiesResolvesRealDocName(t *testing.T) {
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q")

	// A server script whose real name differs from its snake_cased filename
	scriptDir := filepath.Join(dir, "custom", "server_script")
	if err := os.MkdirAll(scriptDir, 0755); err != nil {
		t.Fatal(err)
	}
	doc := map[string]any{
		"name":    "teamplan: auto submit on save",
		"doctype": "Server Script",
		"script":  "pass",
	}
	raw, _ := json.MarshalIndent(doc, "", "  ")
	filePath := filepath.Join(scriptDir, "teamplan:_auto_submit_on_save.json")
	if err := os.WriteFile(filePath, raw, 0644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "-m", "add script")
	baseCommit := gitRun(t, dir, "rev-parse", "HEAD")

	// Record the push baseline, then delete the entity
	if err := os.MkdirAll(filepath.Join(dir, ".weg"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".weg", "last_push_commit"), []byte(baseCommit), 0644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "rm", "-q", "custom/server_script/teamplan:_auto_submit_on_save.json")
	gitRun(t, dir, "commit", "-q", "-m", "remove script")

	entities, err := findDeletedEntities(dir, false)
	if err != nil {
		t.Fatalf("findDeletedEntities failed: %v", err)
	}
	if len(entities) != 1 {
		t.Fatalf("expected 1 deleted entity, got %d", len(entities))
	}
	if entities[0].name != "teamplan: auto submit on save" {
		t.Errorf("expected real doc name from JSON, got %q", entities[0].name)
	}
	if entities[0].doctype != "Server Script" {
		t.Errorf("expected doctype Server Script, got %q", entities[0].doctype)
	}
}

func TestSaveLastPushCommit(t *testing.T) {
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q")
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "-m", "initial")
	head := gitRun(t, dir, "rev-parse", "HEAD")

	if err := os.MkdirAll(filepath.Join(dir, ".weg"), 0755); err != nil {
		t.Fatal(err)
	}
	saveLastPushCommit(dir)

	data, err := os.ReadFile(filepath.Join(dir, ".weg", "last_push_commit"))
	if err != nil {
		t.Fatalf("last_push_commit not written: %v", err)
	}
	if strings.TrimSpace(string(data)) != head {
		t.Errorf("expected baseline %s, got %s", head, data)
	}
}
