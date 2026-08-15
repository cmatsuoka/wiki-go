package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"wiki-go/internal/config"
)

// newTestConfig builds a config pointing at a temp root dir with MCP disabled.
func newTestConfig(t *testing.T) *config.Config {
	t.Helper()
	return &config.Config{
		Wiki: config.WikiConfig{
			RootDir: t.TempDir(),
		},
		MCP: config.MCPConfig{
			Enabled: false,
			Path:    "/mcp",
		},
	}
}

// TestMCPRouteAbsentWhenDisabled verifies that when mcp.enabled is false, the
// /mcp endpoint is not mounted and does not serve MCP JSON-RPC responses.
func TestMCPRouteAbsentWhenDisabled(t *testing.T) {
	cfg := newTestConfig(t)
	SetupRoutes(cfg)

	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","method":"initialize","id":1}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	http.DefaultServeMux.ServeHTTP(w, req)

	// When disabled, the /mcp path falls through to the catch-all handler and
	// must NOT return an MCP JSON-RPC response.
	if strings.Contains(w.Body.String(), `"jsonrpc"`) {
		t.Errorf("expected no MCP JSON-RPC response when disabled, got: %s", w.Body.String())
	}
}
