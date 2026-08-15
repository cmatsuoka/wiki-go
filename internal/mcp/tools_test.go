package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wiki-go/internal/auth"
	"wiki-go/internal/config"
)

// newTestConfig builds a config pointing at a temp root dir.
func newTestConfig(t *testing.T) *config.Config {
	t.Helper()
	root := t.TempDir()
	return &config.Config{
		Wiki: struct {
			RootDir                     string `yaml:"root_dir"`
			DocumentsDir                string `yaml:"documents_dir"`
			Title                       string `yaml:"title"`
			Owner                       string `yaml:"owner"`
			Notice                      string `yaml:"notice"`
			Timezone                    string `yaml:"timezone"`
			Private                     bool   `yaml:"private"`
			DisableComments             bool   `yaml:"disable_comments"`
			DisableFileUploadChecking   bool   `yaml:"disable_file_upload_checking"`
			EnableLinkEmbedding         bool   `yaml:"enable_link_embedding"`
			HideAttachments             bool   `yaml:"hide_attachments"`
			DisableContentMaxWidth      bool   `yaml:"disable_content_max_width"`
			AlwaysOpenChildrenInSidebar bool   `yaml:"always_open_children_in_sidebar"`
			MaxVersions                 int    `yaml:"max_versions"`
			MaxUploadSize               int    `yaml:"max_upload_size"`
			Language                    string `yaml:"language"`
			LogLevel                    string `yaml:"log_level"`
		}{
			RootDir:      root,
			DocumentsDir: "documents",
			MaxVersions:  10,
		},
	}
}

// editorSession returns a session with editor role.
func editorSession() *auth.Session {
	return &auth.Session{Username: "tester", Role: "editor"}
}

// callTool drives a tools/call request through the handler and returns the result.
func callTool(t *testing.T, h *Handler, name string, args string) toolResult {
	t.Helper()

	// Initialize to get a session
	initReq := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","method":"initialize","id":1}`))
	initReq.Header.Set("Content-Type", "application/json")
	initReq = injectSession(initReq, &sessionEntry{AuthSession: editorSession()})
	initW := httptest.NewRecorder()
	h.ServeHTTP(initW, initReq)
	sessionID := initW.Header().Get("Mcp-Session-Id")

	body := `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"` + name + `","arguments":` + args + `},"id":2}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Mcp-Session-Id", sessionID)
	req = injectSession(req, &sessionEntry{AuthSession: editorSession()})

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	var resp jsonrpcResponse
	if err := json.Unmarshal([]byte(extractJSONFromSSE(w.Body.String())), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected JSON-RPC error: %+v", resp.Error)
	}

	b, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}
	var tr toolResult
	if err := json.Unmarshal(b, &tr); err != nil {
		t.Fatalf("failed to unmarshal tool result: %v", err)
	}
	return tr
}

func TestToolsListContainsAllTools(t *testing.T) {
	h := NewHandler(newTestConfig(t))
	defer h.Close()

	initReq := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","method":"initialize","id":1}`))
	initReq.Header.Set("Content-Type", "application/json")
	initReq = injectSession(initReq, &sessionEntry{AuthSession: editorSession()})
	initW := httptest.NewRecorder()
	h.ServeHTTP(initW, initReq)
	sessionID := initW.Header().Get("Mcp-Session-Id")

	listReq := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","method":"tools/list","id":2}`))
	listReq.Header.Set("Content-Type", "application/json")
	listReq.Header.Set("Mcp-Session-Id", sessionID)
	listReq = injectSession(listReq, &sessionEntry{AuthSession: editorSession()})
	listW := httptest.NewRecorder()
	h.ServeHTTP(listW, listReq)

	var resp jsonrpcResponse
	if err := json.Unmarshal([]byte(extractJSONFromSSE(listW.Body.String())), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("expected result to be a map")
	}
	tools, ok := result["tools"].([]interface{})
	if !ok {
		t.Fatal("expected tools to be a list")
	}

	names := map[string]bool{}
	for _, t := range tools {
		tm, ok := t.(map[string]interface{})
		if !ok {
			continue
		}
		if n, ok := tm["name"].(string); ok {
			names[n] = true
		}
	}

	for _, want := range []string{"read_page", "list_pages", "search_pages", "create_page", "write_page", "delete_page"} {
		if !names[want] {
			t.Errorf("expected tool %q in tools/list", want)
		}
	}
}

func TestCreateReadWriteDeleteCycle(t *testing.T) {
	cfg := newTestConfig(t)
	h := NewHandler(cfg)
	defer h.Close()

	// create_page
	res := callTool(t, h, "create_page", `{"title":"Test Page","path":"test-page","type":"markdown"}`)
	if res.IsError {
		t.Fatalf("create_page failed: %s", res.Content[0].Text)
	}

	// read_page
	res = callTool(t, h, "read_page", `{"path":"test-page"}`)
	if res.IsError {
		t.Fatalf("read_page failed: %s", res.Content[0].Text)
	}
	if !strings.Contains(res.Content[0].Text, "Test Page") {
		t.Errorf("expected content to contain 'Test Page', got %q", res.Content[0].Text)
	}

	// write_page
	res = callTool(t, h, "write_page", `{"path":"test-page","content":"# Updated\n\nNew content"}`)
	if res.IsError {
		t.Fatalf("write_page failed: %s", res.Content[0].Text)
	}

	// read_page again to confirm write
	res = callTool(t, h, "read_page", `{"path":"test-page"}`)
	if res.IsError {
		t.Fatalf("read_page failed: %s", res.Content[0].Text)
	}
	if !strings.Contains(res.Content[0].Text, "New content") {
		t.Errorf("expected updated content, got %q", res.Content[0].Text)
	}

	// list_pages
	res = callTool(t, h, "list_pages", `{}`)
	if res.IsError {
		t.Fatalf("list_pages failed: %s", res.Content[0].Text)
	}
	if !strings.Contains(res.Content[0].Text, "test-page") {
		t.Errorf("expected list to contain test-page, got %q", res.Content[0].Text)
	}

	// search_pages
	res = callTool(t, h, "search_pages", `{"query":"New content"}`)
	if res.IsError {
		t.Fatalf("search_pages failed: %s", res.Content[0].Text)
	}
	if !strings.Contains(res.Content[0].Text, "test-page") {
		t.Errorf("expected search to find test-page, got %q", res.Content[0].Text)
	}

	// delete_page
	res = callTool(t, h, "delete_page", `{"path":"test-page"}`)
	if res.IsError {
		t.Fatalf("delete_page failed: %s", res.Content[0].Text)
	}

	// Confirm deletion
	if _, err := os.Stat(filepath.Join(cfg.Wiki.RootDir, "documents", "test-page")); !os.IsNotExist(err) {
		t.Errorf("expected test-page directory to be removed, got err %v", err)
	}
}

func TestReadPageNotFound(t *testing.T) {
	h := NewHandler(newTestConfig(t))
	defer h.Close()

	res := callTool(t, h, "read_page", `{"path":"does-not-exist"}`)
	if !res.IsError {
		t.Fatalf("expected error for missing page, got %s", res.Content[0].Text)
	}
	if !strings.Contains(res.Content[0].Text, "not found") {
		t.Errorf("expected 'not found' in error, got %q", res.Content[0].Text)
	}
}

func TestDeleteHomepageRefused(t *testing.T) {
	h := NewHandler(newTestConfig(t))
	defer h.Close()

	res := callTool(t, h, "delete_page", `{"path":""}`)
	if !res.IsError {
		t.Fatalf("expected error deleting homepage, got %s", res.Content[0].Text)
	}
}

func TestCreatePageConflict(t *testing.T) {
	cfg := newTestConfig(t)
	h := NewHandler(cfg)
	defer h.Close()

	res := callTool(t, h, "create_page", `{"title":"A","path":"dup","type":"markdown"}`)
	if res.IsError {
		t.Fatalf("first create failed: %s", res.Content[0].Text)
	}

	res = callTool(t, h, "create_page", `{"title":"B","path":"dup","type":"markdown"}`)
	if !res.IsError {
		t.Fatalf("expected conflict on duplicate create, got %s", res.Content[0].Text)
	}
	if !strings.Contains(res.Content[0].Text, "already exists") {
		t.Errorf("expected 'already exists' in error, got %q", res.Content[0].Text)
	}
}

func TestUnauthorizedRoleRejected(t *testing.T) {
	cfg := newTestConfig(t)
	h := NewHandler(cfg)
	defer h.Close()

	// A viewer session should be rejected by the service role check.
	initReq := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","method":"initialize","id":1}`))
	initReq.Header.Set("Content-Type", "application/json")
	initReq = injectSession(initReq, &sessionEntry{AuthSession: &auth.Session{Username: "viewer", Role: "viewer"}})
	initW := httptest.NewRecorder()
	h.ServeHTTP(initW, initReq)
	sessionID := initW.Header().Get("Mcp-Session-Id")

	body := `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"list_pages","arguments":{}},"id":2}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Mcp-Session-Id", sessionID)
	req = injectSession(req, &sessionEntry{AuthSession: &auth.Session{Username: "viewer", Role: "viewer"}})

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	var resp jsonrpcResponse
	if err := json.Unmarshal([]byte(extractJSONFromSSE(w.Body.String())), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	b, _ := json.Marshal(resp.Result)
	var tr toolResult
	json.Unmarshal(b, &tr)
	if !tr.IsError {
		t.Fatalf("expected error for viewer role, got %s", tr.Content[0].Text)
	}
	if !strings.Contains(tr.Content[0].Text, "Unauthorized") {
		t.Errorf("expected 'Unauthorized' in error, got %q", tr.Content[0].Text)
	}
}
