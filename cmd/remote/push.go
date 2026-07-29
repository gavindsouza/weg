/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package remote

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/prompt"
	"github.com/gavindsouza/weg/internal/remote"
	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push local changes to the remote site",
	Long: `Push local file changes to the remote Frappe site.

By default, only pushes committed changes since last push.
Use -u to include uncommitted changes, or -a to push everything.

Examples:
  weg remote push           # Push committed changes since last push
  weg remote push -n        # Dry-run (preview what would be pushed)
  weg remote push -u        # Include uncommitted changes
  weg remote push -a        # Push all entities (use with caution)
  weg remote push -f        # Force push even if remote is newer`,
	RunE: runPush,
}

var (
	pushDryRun      bool
	pushForce       bool
	pushAll         bool
	pushUncommitted bool
)

func init() {
	pushCmd.Flags().BoolVarP(&pushDryRun, "dry-run", "n", false, "Preview changes without pushing")
	pushCmd.Flags().BoolVarP(&pushForce, "force", "f", false, "Force push even if remote is newer")
	pushCmd.Flags().BoolVarP(&pushAll, "all", "a", false, "Push all entities (not just changed ones)")
	pushCmd.Flags().BoolVarP(&pushUncommitted, "uncommitted", "u", false, "Include uncommitted changes")
}

func runPush(cobraCmd *cobra.Command, args []string) error {
	// Check if we're in a remote site directory
	if !remote.IsRemoteSite(".") {
		return wegerrors.NotFound("remote clone", ".weg/site.toml")
	}

	// Load config and credentials
	config, err := remote.LoadSiteConfig(".")
	if err != nil {
		return wegerrors.Config("site.toml", "read", err)
	}

	creds, err := remote.LoadCredentials(".")
	if err != nil {
		return wegerrors.Config("credentials", "read", err)
	}

	// Find modified files using git diff
	var entities []localEntity

	if pushAll {
		// Push everything (dangerous, but sometimes needed)
		var err error
		entities, err = findLocalEntities(".")
		if err != nil {
			return fmt.Errorf("failed to find entities: %w", err)
		}
	} else {
		// Only push changed files (default, safe behavior)
		var err error
		entities, err = findChangedEntities(".", pushUncommitted)
		if err != nil {
			return fmt.Errorf("failed to find changed entities: %w", err)
		}
	}

	if len(entities) == 0 {
		output.Print("No changes to push")
		output.Print("(use --all to push all entities)")
		return nil
	}

	// Find deleted entities for dry-run display
	deletedEntities, _ := findDeletedEntities(".", pushUncommitted)

	if pushDryRun {
		if pushAll {
			output.Printf("Dry run - would push ALL %d entities:", len(entities))
		} else {
			output.Printf("Dry run - would push %d changed entities:", len(entities))
		}
		for _, e := range entities {
			output.Printf("  + %s: %s", e.entityType, e.name)
		}
		if len(deletedEntities) > 0 {
			output.Printf("\nWould delete %d entities:", len(deletedEntities))
			for _, e := range deletedEntities {
				output.Printf("  - %s: %s", e.entityType, e.name)
			}
		}
		return nil
	}

	// Connect
	output.Infof("Connecting to %s...\n", config.Site.URL)
	client := remote.NewClientFromConfig(config, creds)
	if err := client.Ping(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	output.Print("Connected")

	// Push each entity
	totalChanges := len(entities) + len(deletedEntities)
	if totalChanges == 0 {
		output.Print("No changes to push")
		return nil
	}

	output.Infof("Pushing %d changes...\n", totalChanges)
	var stats pushStats
	deleted := 0
	failed := 0

	for _, e := range entities {
		s, err := pushEntity(client, e, pushForce)
		stats.add(s)
		if err != nil {
			output.Errorf("Failed to push %s: %v", e.name, err)
			failed++
		}
	}

	// Delete removed entities (with confirmation)
	if len(deletedEntities) > 0 && !pushForce {
		output.Printf("\nThe following %d entities will be deleted from the remote:", len(deletedEntities))
		for _, e := range deletedEntities {
			output.Printf("  - %s: %s", e.entityType, e.name)
		}
		if !prompt.ConfirmDanger("Delete these entities from remote?") {
			output.Print("Skipping deletions")
			deletedEntities = nil
		}
	}

	for _, e := range deletedEntities {
		if err := deleteEntity(client, e); err != nil {
			output.Errorf("Failed to delete %s: %v", e.name, err)
			failed++
		} else {
			deleted++
		}
	}

	output.Printf("Pushed: %d, Up-to-date: %d, Deleted: %d, Failed: %d",
		stats.created+stats.updated, stats.skipped, deleted, failed)

	if failed > 0 {
		return wegerrors.Operation("push", fmt.Sprintf("%d entities failed", failed), nil)
	}

	// Save current commit as last push point
	if !pushDryRun {
		saveLastPushCommit(".")
	}

	return nil
}

// saveLastPushCommit saves the current HEAD commit as the last push point
func saveLastPushCommit(baseDir string) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = baseDir
	output, err := cmd.Output()
	if err != nil {
		return
	}

	commit := strings.TrimSpace(string(output))
	lastPushFile := filepath.Join(baseDir, ".weg", "last_push_commit")
	os.WriteFile(lastPushFile, []byte(commit), 0644)
}

type localEntity struct {
	filePath   string
	entityType string
	doctype    string
	name       string
	data       map[string]any
}

// findChangedEntities finds only entities that have been modified since last push
func findChangedEntities(baseDir string, includeUncommitted bool) ([]localEntity, error) {
	// Get changed files from git
	changedFiles, err := getChangedFiles(baseDir, includeUncommitted)
	if err != nil {
		return nil, fmt.Errorf("failed to get changed files: %w", err)
	}

	if len(changedFiles) == 0 {
		return nil, nil
	}

	var entities []localEntity
	seen := make(map[string]bool) // Dedupe by file path

	for _, filePath := range changedFiles {
		// Skip non-JSON files and special files
		if !strings.HasSuffix(filePath, ".json") {
			continue
		}
		if strings.HasPrefix(filePath, ".weg/") || strings.HasPrefix(filePath, "weg_workspace/") {
			continue
		}

		// Skip if already processed
		if seen[filePath] {
			continue
		}
		seen[filePath] = true

		// Read and parse the file
		fullPath := filepath.Join(baseDir, filePath)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			// File might have been deleted
			continue
		}

		var doc map[string]any
		if err := json.Unmarshal(data, &doc); err != nil {
			continue
		}

		// Detect entity type from path
		parts := strings.Split(filePath, string(filepath.Separator))
		if len(parts) < 2 {
			continue
		}

		// Find the entity type directory (e.g., "server_script", "client_script")
		var typeName string
		for _, part := range parts {
			if isEntityType(part) {
				typeName = part
				break
			}
		}
		if typeName == "" {
			continue
		}

		doctype := typeToDocType(typeName)
		name := getString(doc, "name")
		if name == "" {
			name = strings.TrimSuffix(filepath.Base(filePath), ".json")
		}

		entities = append(entities, localEntity{
			filePath:   fullPath,
			entityType: typeName,
			doctype:    doctype,
			name:       name,
			data:       doc,
		})
	}

	return entities, nil
}

// getChangedFiles returns list of changed files using git
func getChangedFiles(baseDir string, includeUncommitted bool) ([]string, error) {
	var allFiles []string

	if includeUncommitted {
		// Get uncommitted changes (staged and unstaged)
		cmd := exec.Command("git", "diff", "--name-only", "HEAD")
		cmd.Dir = baseDir
		output, err := cmd.Output()
		if err == nil && len(output) > 0 {
			files := strings.Split(strings.TrimSpace(string(output)), "\n")
			allFiles = append(allFiles, files...)
		}

		// Get staged changes
		cmd = exec.Command("git", "diff", "--name-only", "--cached")
		cmd.Dir = baseDir
		output, err = cmd.Output()
		if err == nil && len(output) > 0 {
			files := strings.Split(strings.TrimSpace(string(output)), "\n")
			allFiles = append(allFiles, files...)
		}
	}

	// Get committed changes since last push
	// Uses .weg/last_push_commit to know what was last pushed
	lastPushFile := filepath.Join(baseDir, ".weg", "last_push_commit")
	if data, err := os.ReadFile(lastPushFile); err == nil {
		lastCommit := strings.TrimSpace(string(data))
		if lastCommit != "" {
			cmd := exec.Command("git", "diff", "--name-only", lastCommit+"..HEAD")
			cmd.Dir = baseDir
			output, err := cmd.Output()
			if err == nil && len(output) > 0 {
				files := strings.Split(strings.TrimSpace(string(output)), "\n")
				allFiles = append(allFiles, files...)
			}
		}
	} else {
		// No last push commit, get changes from initial commit to HEAD
		// This handles fresh clones - compare against root commit
		cmd := exec.Command("git", "rev-list", "--max-parents=0", "HEAD")
		cmd.Dir = baseDir
		output, err := cmd.Output()
		if err == nil && len(output) > 0 {
			rootCommit := strings.TrimSpace(strings.Split(string(output), "\n")[0])
			cmd = exec.Command("git", "diff", "--name-only", rootCommit+"..HEAD")
			cmd.Dir = baseDir
			output, err = cmd.Output()
			if err == nil && len(output) > 0 {
				files := strings.Split(strings.TrimSpace(string(output)), "\n")
				allFiles = append(allFiles, files...)
			}
		}
	}

	// Dedupe
	seen := make(map[string]bool)
	var result []string
	for _, f := range allFiles {
		if f != "" && !seen[f] {
			seen[f] = true
			result = append(result, f)
		}
	}

	return result, nil
}

// findDeletedEntities finds entities that were deleted since last push
func findDeletedEntities(baseDir string, includeUncommitted bool) ([]localEntity, error) {
	var entities []localEntity

	// Get deleted files from git, remembering the revision that still has each
	// file so its real document name can be read back from the JSON content
	var deletedFiles []string
	fileRev := make(map[string]string)

	if includeUncommitted {
		// Check for deleted files in working tree
		cmd := exec.Command("git", "diff", "--name-status", "HEAD")
		cmd.Dir = baseDir
		output, err := cmd.Output()
		if err == nil {
			lines := strings.Split(strings.TrimSpace(string(output)), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "D\t") {
					file := strings.TrimPrefix(line, "D\t")
					deletedFiles = append(deletedFiles, file)
					fileRev[file] = "HEAD"
				}
			}
		}
	}

	// Check for deleted files since last push
	lastPushFile := filepath.Join(baseDir, ".weg", "last_push_commit")
	if data, err := os.ReadFile(lastPushFile); err == nil {
		lastCommit := strings.TrimSpace(string(data))
		if lastCommit != "" {
			cmd := exec.Command("git", "diff", "--name-status", lastCommit+"..HEAD")
			cmd.Dir = baseDir
			output, err := cmd.Output()
			if err == nil {
				lines := strings.Split(strings.TrimSpace(string(output)), "\n")
				for _, line := range lines {
					if strings.HasPrefix(line, "D\t") {
						file := strings.TrimPrefix(line, "D\t")
						deletedFiles = append(deletedFiles, file)
						if _, ok := fileRev[file]; !ok {
							fileRev[file] = lastCommit
						}
					}
				}
			}
		}
	}

	// Dedupe and process
	seen := make(map[string]bool)
	for _, filePath := range deletedFiles {
		if seen[filePath] || !strings.HasSuffix(filePath, ".json") {
			continue
		}
		if strings.HasPrefix(filePath, ".weg/") || strings.HasPrefix(filePath, "weg_workspace/") {
			continue
		}
		seen[filePath] = true

		// Detect entity type from path
		parts := strings.Split(filePath, string(filepath.Separator))
		var typeName string
		for _, part := range parts {
			if isEntityType(part) {
				typeName = part
				break
			}
		}
		if typeName == "" {
			continue
		}

		// File names are snake_cased on disk, so the real document name (which
		// may contain spaces, colons, uppercase) must come from the deleted
		// file's JSON content in git history
		name := docNameFromGit(baseDir, fileRev[filePath], filePath)
		if name == "" {
			// Fallback: filename without extension
			name = strings.TrimSuffix(filepath.Base(filePath), ".json")
		}

		entities = append(entities, localEntity{
			filePath:   filePath,
			entityType: typeName,
			doctype:    typeToDocType(typeName),
			name:       name,
		})
	}

	return entities, nil
}

// docNameFromGit reads a file's JSON content at the given revision and returns
// its "name" field. Returns "" if the file or field can't be read.
func docNameFromGit(baseDir, rev, filePath string) string {
	if rev == "" {
		return ""
	}
	cmd := exec.Command("git", "show", rev+":"+filePath)
	cmd.Dir = baseDir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		return ""
	}
	return getString(doc, "name")
}

// deleteEntity deletes an entity on the remote site
func deleteEntity(client *remote.Client, e localEntity) error {
	switch e.entityType {
	case "custom_field", "property_setter":
		// These are grouped files - can't delete individual items this way
		// Would need special handling
		return wegerrors.Validation("entity", "deletion of grouped entities not yet supported")
	default:
		return client.DeleteDoc(e.doctype, e.name)
	}
}

// isEntityType checks if a directory name is an entity type
func isEntityType(name string) bool {
	types := []string{
		"doctype", "custom_field", "property_setter", "client_script",
		"server_script", "report", "print_format", "workflow",
		"notification", "letter_head", "web_template",
	}
	for _, t := range types {
		if name == t {
			return true
		}
	}
	return false
}

func findLocalEntities(baseDir string) ([]localEntity, error) {
	var entities []localEntity

	// Walk through module directories
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		modulePath := filepath.Join(baseDir, entry.Name())
		moduleEntries, err := os.ReadDir(modulePath)
		if err != nil {
			continue
		}

		for _, typeEntry := range moduleEntries {
			if !typeEntry.IsDir() {
				continue
			}

			typeName := typeEntry.Name()
			typePath := filepath.Join(modulePath, typeName)

			// Find JSON files
			err := filepath.Walk(typePath, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() || !strings.HasSuffix(path, ".json") {
					return nil
				}

				data, err := os.ReadFile(path)
				if err != nil {
					return nil
				}

				var doc map[string]any
				if err := json.Unmarshal(data, &doc); err != nil {
					return nil
				}

				// Determine doctype based on directory
				doctype := typeToDocType(typeName)
				name := getString(doc, "name")
				if name == "" {
					// Try to get from filename
					name = strings.TrimSuffix(filepath.Base(path), ".json")
				}

				entities = append(entities, localEntity{
					filePath:   path,
					entityType: typeName,
					doctype:    doctype,
					name:       name,
					data:       doc,
				})

				return nil
			})
			if err != nil {
				continue
			}
		}
	}

	return entities, nil
}

func typeToDocType(typeName string) string {
	switch typeName {
	case "doctype":
		return "DocType"
	case "custom_field":
		return "Custom Field"
	case "property_setter":
		return "Property Setter"
	case "client_script":
		return "Client Script"
	case "server_script":
		return "Server Script"
	case "report":
		return "Report"
	case "print_format":
		return "Print Format"
	case "workflow":
		return "Workflow"
	case "notification":
		return "Notification"
	case "letter_head":
		return "Letter Head"
	case "web_template":
		return "Web Template"
	default:
		return typeName
	}
}

// pushOutcome describes what pushing a single document did
type pushOutcome int

const (
	outcomeCreated pushOutcome = iota
	outcomeUpdated
	outcomeSkipped
)

// pushStats counts push outcomes across documents
type pushStats struct {
	created int
	updated int
	skipped int
}

func (s *pushStats) add(o pushStats) {
	s.created += o.created
	s.updated += o.updated
	s.skipped += o.skipped
}

func pushEntity(client *remote.Client, e localEntity, force bool) (pushStats, error) {
	// Handle special cases
	switch e.entityType {
	case "custom_field":
		return pushGrouped(client, e, "custom_fields", "Custom Field", force)
	case "property_setter":
		return pushGrouped(client, e, "property_setters", "Property Setter", force)
	default:
		return pushDocument(client, e, force)
	}
}

// pushDoc creates or updates a single document on the remote and returns the
// saved document as the server stored it.
//
// Only a 404 from the existence check is treated as "create"; any other error
// (auth, network, server) aborts instead of being masked as an insert.
// Unchanged documents are skipped so re-pushing already-synced state doesn't
// rewrite `modified` and pollute the site's version history. When the remote
// changed since the local base (modified mismatch) the push is refused unless
// force is set, instead of silently overwriting remote edits.
func pushDoc(client *remote.Client, doctype, name string, data map[string]any, force bool) (map[string]any, pushOutcome, error) {
	existing, err := client.GetDoc(doctype, name)
	if err != nil {
		if !remote.IsNotFound(err) {
			return nil, outcomeSkipped, fmt.Errorf("failed to fetch %s %q: %w", doctype, name, err)
		}
		// Doesn't exist, create it
		saved, err := client.InsertDoc(doctype, data)
		if err != nil {
			return nil, outcomeSkipped, err
		}
		return saved, outcomeCreated, nil
	}

	// Already in sync (ignoring volatile fields): nothing to do
	if docsEqual(data, existing) {
		return existing, outcomeSkipped, nil
	}

	// The local `modified` is the remote timestamp at last sync. If the remote
	// has moved past it, someone changed the document on the site since then.
	localMod := getString(data, "modified")
	remoteMod := getString(existing, "modified")
	if !force && localMod != "" && remoteMod != "" && localMod != remoteMod {
		return nil, outcomeSkipped, fmt.Errorf(
			"%s %q changed on remote since last sync (local base %s, remote %s); pull first or use --force",
			doctype, name, localMod, remoteMod)
	}

	// Copy the server's modified timestamp so the update passes Frappe's
	// timestamp check (check_if_latest)
	if remoteMod != "" {
		data["modified"] = remoteMod
	}

	saved, err := client.UpdateDoc(doctype, name, data)
	if err != nil {
		return nil, outcomeSkipped, err
	}
	return saved, outcomeUpdated, nil
}

func pushDocument(client *remote.Client, e localEntity, force bool) (pushStats, error) {
	var stats pushStats
	saved, outcome, err := pushDoc(client, e.doctype, e.name, e.data, force)
	if err != nil {
		return stats, err
	}

	switch outcome {
	case outcomeCreated:
		stats.created++
	case outcomeUpdated:
		stats.updated++
	case outcomeSkipped:
		stats.skipped++
		return stats, nil
	}

	// Track the saved document (notably its new `modified`) locally, so the
	// next push doesn't misread our own change as a remote conflict.
	writeLocalDoc(e.filePath, saved)
	return stats, nil
}

// pushGrouped pushes a grouped file (custom fields / property setters keyed by
// target doctype) one row at a time.
func pushGrouped(client *remote.Client, e localEntity, key, doctype string, force bool) (pushStats, error) {
	var stats pushStats
	rows, ok := e.data[key].([]any)
	if !ok {
		return stats, wegerrors.Validation(key, "invalid format")
	}

	changed := false
	for i, r := range rows {
		row, ok := r.(map[string]any)
		if !ok {
			continue
		}

		name := getString(row, "name")
		var saved map[string]any
		var outcome pushOutcome
		var err error
		if name == "" {
			// New row, insert
			saved, err = client.InsertDoc(doctype, row)
			outcome = outcomeCreated
		} else {
			saved, outcome, err = pushDoc(client, doctype, name, row, force)
		}
		if err != nil {
			if changed {
				writeLocalDoc(e.filePath, e.data)
			}
			return stats, err
		}

		switch outcome {
		case outcomeCreated:
			stats.created++
		case outcomeUpdated:
			stats.updated++
		case outcomeSkipped:
			stats.skipped++
			continue
		}
		if saved != nil {
			rows[i] = saved
			changed = true
		}
	}

	if changed {
		writeLocalDoc(e.filePath, e.data)
	}
	return stats, nil
}

// docsEqual reports whether two documents match, ignoring fields the server
// rewrites on every save (modified, modified_by) and server-only "__" keys.
func docsEqual(local, remoteDoc map[string]any) bool {
	return string(normalizeDoc(local)) == string(normalizeDoc(remoteDoc))
}

func normalizeDoc(doc map[string]any) []byte {
	clean := make(map[string]any, len(doc))
	for k, v := range doc {
		if k == "modified" || k == "modified_by" || strings.HasPrefix(k, "__") {
			continue
		}
		clean[k] = v
	}
	data, _ := json.Marshal(clean)
	return data
}

// writeLocalDoc rewrites a local JSON file with the document the server
// returned, keeping the local base in step with the remote after a push.
func writeLocalDoc(path string, doc map[string]any) {
	if path == "" || doc == nil {
		return
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(path, data, 0644)
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
