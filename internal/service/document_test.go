package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"wiki-go/internal/auth"
	"wiki-go/internal/config"
	"wiki-go/internal/roles"
)

// newTestConfig builds a config pointing at a temp root dir.
func newTestConfig(t *testing.T) *config.Config {
	t.Helper()
	root := t.TempDir()
	return &config.Config{
		Wiki: config.WikiConfig{
			RootDir:      root,
			DocumentsDir: "documents",
			MaxVersions:  10,
		},
	}
}

func editorSession() *auth.Session {
	return &auth.Session{Username: "tester", Role: roles.RoleEditor}
}

func adminSession() *auth.Session {
	return &auth.Session{Username: "admin", Role: roles.RoleAdmin}
}

func viewerSession() *auth.Session {
	return &auth.Session{Username: "viewer", Role: roles.RoleViewer}
}

func TestCreateReadWriteListSearchDeleteCycle(t *testing.T) {
	cfg := newTestConfig(t)
	sess := editorSession()

	// create
	url, err := Create(cfg, sess, CreateDocumentRequest{Title: "Test Page", Path: "test-page", Type: "markdown"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if url != "/test-page" {
		t.Errorf("expected url /test-page, got %q", url)
	}

	// read
	content, err := GetSource(cfg, sess, "test-page")
	if err != nil {
		t.Fatalf("GetSource failed: %v", err)
	}
	if !strings.Contains(content, "Test Page") {
		t.Errorf("expected content to contain 'Test Page', got %q", content)
	}

	// write
	if err := Save(cfg, sess, "test-page", "# Updated\n\nNew content"); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// read again
	content, err = GetSource(cfg, sess, "test-page")
	if err != nil {
		t.Fatalf("GetSource after save failed: %v", err)
	}
	if !strings.Contains(content, "New content") {
		t.Errorf("expected updated content, got %q", content)
	}

	// list
	docs, err := List(cfg, sess)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	found := false
	for _, d := range docs {
		if d.Path == "/test-page" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected list to contain /test-page, got %+v", docs)
	}

	// search
	results := Search(cfg, sess, "New content")
	if len(results) == 0 {
		t.Fatal("expected search to return results")
	}
	if !strings.HasPrefix(results[0].Path, "/test-page") {
		t.Errorf("expected search result path to start with /test-page, got %q", results[0].Path)
	}

	// delete
	if err := Delete(cfg, sess, "test-page", true); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.Wiki.RootDir, "documents", "test-page")); !os.IsNotExist(err) {
		t.Errorf("expected test-page directory to be removed, got err %v", err)
	}
}

func TestRoleEnforcement(t *testing.T) {
	cfg := newTestConfig(t)

	// viewer cannot create
	if _, err := Create(cfg, viewerSession(), CreateDocumentRequest{Title: "A", Path: "a", Type: "markdown"}); err != ErrUnauthorized {
		t.Errorf("expected ErrUnauthorized for viewer create, got %v", err)
	}

	// viewer cannot read
	if _, err := GetSource(cfg, viewerSession(), "a"); err != ErrUnauthorized {
		t.Errorf("expected ErrUnauthorized for viewer read, got %v", err)
	}

	// viewer cannot list
	if _, err := List(cfg, viewerSession()); err != ErrUnauthorized {
		t.Errorf("expected ErrUnauthorized for viewer list, got %v", err)
	}

	// viewer cannot save
	if err := Save(cfg, viewerSession(), "a", "x"); err != ErrUnauthorized {
		t.Errorf("expected ErrUnauthorized for viewer save, got %v", err)
	}

	// viewer cannot delete
	if err := Delete(cfg, viewerSession(), "a", true); err != ErrUnauthorized {
		t.Errorf("expected ErrUnauthorized for viewer delete, got %v", err)
	}

	// nil session rejected
	if _, err := List(cfg, nil); err != ErrUnauthorized {
		t.Errorf("expected ErrUnauthorized for nil session, got %v", err)
	}

	// admin can create
	if _, err := Create(cfg, adminSession(), CreateDocumentRequest{Title: "A", Path: "a", Type: "markdown"}); err != nil {
		t.Errorf("expected admin create to succeed, got %v", err)
	}
}

func TestGetSourcePlaceholder(t *testing.T) {
	cfg := newTestConfig(t)
	sess := editorSession()

	// Create a directory without document.md
	dir := filepath.Join(cfg.Wiki.RootDir, "documents", "empty-page")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	content, err := GetSource(cfg, sess, "empty-page")
	if err != nil {
		t.Fatalf("expected placeholder content, got error %v", err)
	}
	if !strings.Contains(content, "Enter content here") {
		t.Errorf("expected placeholder content, got %q", content)
	}
}

func TestGetSourceNotFound(t *testing.T) {
	cfg := newTestConfig(t)
	sess := editorSession()

	if _, err := GetSource(cfg, sess, "does-not-exist"); err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDeleteHomepageRefused(t *testing.T) {
	cfg := newTestConfig(t)
	sess := editorSession()

	if err := Delete(cfg, sess, "", true); err != ErrNotFound {
		t.Errorf("expected ErrNotFound deleting homepage, got %v", err)
	}
	if err := Delete(cfg, sess, "/", true); err != ErrNotFound {
		t.Errorf("expected ErrNotFound deleting homepage, got %v", err)
	}
}

func TestCreateConflict(t *testing.T) {
	cfg := newTestConfig(t)
	sess := editorSession()

	if _, err := Create(cfg, sess, CreateDocumentRequest{Title: "A", Path: "dup", Type: "markdown"}); err != nil {
		t.Fatalf("first create failed: %v", err)
	}
	if _, err := Create(cfg, sess, CreateDocumentRequest{Title: "B", Path: "dup", Type: "markdown"}); err != ErrConflict {
		t.Errorf("expected ErrConflict on duplicate create, got %v", err)
	}
}

func TestDeleteWithChildren(t *testing.T) {
	cfg := newTestConfig(t)
	sess := editorSession()

	if _, err := Create(cfg, sess, CreateDocumentRequest{Title: "Parent", Path: "parent", Type: "markdown"}); err != nil {
		t.Fatalf("create parent failed: %v", err)
	}
	if _, err := Create(cfg, sess, CreateDocumentRequest{Title: "Child", Path: "parent/child", Type: "markdown"}); err != nil {
		t.Fatalf("create child failed: %v", err)
	}

	// protectChildren=true refuses
	if err := Delete(cfg, sess, "parent", true); err != ErrHasChildren {
		t.Errorf("expected ErrHasChildren, got %v", err)
	}

	// protectChildren=false allows
	if err := Delete(cfg, sess, "parent", false); err != nil {
		t.Errorf("expected delete to succeed, got %v", err)
	}
}

func TestSaveCreatesVersion(t *testing.T) {
	cfg := newTestConfig(t)
	sess := editorSession()

	// save v1
	if err := Save(cfg, sess, "ver-page", "# v1"); err != nil {
		t.Fatalf("first save failed: %v", err)
	}
	time.Sleep(1 * time.Second)
	// save v2 (v1 should be archived)
	if err := Save(cfg, sess, "ver-page", "# v2"); err != nil {
		t.Fatalf("second save failed: %v", err)
	}
	time.Sleep(1 * time.Second)
	// save v3 (v2 should be archived)
	if err := Save(cfg, sess, "ver-page", "# v3"); err != nil {
		t.Fatalf("third save failed: %v", err)
	}

	versionDir := filepath.Join(cfg.Wiki.RootDir, "versions", "documents", "ver-page")
	entries, err := os.ReadDir(versionDir)
	if err != nil {
		t.Fatalf("expected versions dir to exist: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 version files (v1 and v2), got %d", len(entries))
	}
}

func TestMove(t *testing.T) {
	cfg := newTestConfig(t)
	sess := editorSession()

	if _, err := Create(cfg, sess, CreateDocumentRequest{Title: "Old", Path: "old", Type: "markdown"}); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	if err := Move(cfg, sess, "old", "new"); err != nil {
		t.Fatalf("Move failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(cfg.Wiki.RootDir, "documents", "new", "document.md")); err != nil {
		t.Errorf("expected new document to exist, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.Wiki.RootDir, "documents", "old")); !os.IsNotExist(err) {
		t.Errorf("expected old document to be gone, got %v", err)
	}
}

func TestMoveHomepageRefused(t *testing.T) {
	cfg := newTestConfig(t)
	sess := editorSession()

	if err := Move(cfg, sess, "", "new"); err != ErrNotFound {
		t.Errorf("expected ErrNotFound moving homepage, got %v", err)
	}
}

func TestResolveDocPath(t *testing.T) {
	cfg := newTestConfig(t)

	// homepage
	docPath, dirPath, rel := ResolveDocPath(cfg, "")
	if docPath != filepath.Join(cfg.Wiki.RootDir, "pages", "home", "document.md") {
		t.Errorf("unexpected homepage docPath %q", docPath)
	}
	if dirPath != filepath.Join(cfg.Wiki.RootDir, "pages", "home") {
		t.Errorf("unexpected homepage dirPath %q", dirPath)
	}
	if rel != "pages/home" {
		t.Errorf("unexpected homepage rel %q", rel)
	}

	// regular page
	docPath, dirPath, rel = ResolveDocPath(cfg, "my-page")
	if docPath != filepath.Join(cfg.Wiki.RootDir, "documents", "my-page", "document.md") {
		t.Errorf("unexpected docPath %q", docPath)
	}
	if dirPath != filepath.Join(cfg.Wiki.RootDir, "documents", "my-page") {
		t.Errorf("unexpected dirPath %q", dirPath)
	}
	if rel != "documents/my-page" {
		t.Errorf("unexpected rel %q", rel)
	}
}
