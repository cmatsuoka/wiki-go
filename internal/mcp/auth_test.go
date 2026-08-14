package mcp

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"wiki-go/internal/auth"
	"wiki-go/internal/config"
)

func TestExtractToken(t *testing.T) {
	tests := []struct {
		name     string
		authHdr  string
		tokenHdr string
		want     string
	}{
		{
			name:     "bearer token",
			authHdr:  "Bearer my-secret-token",
			tokenHdr: "",
			want:     "my-secret-token",
		},
		{
			name:     "X-MCP-Token header",
			authHdr:  "",
			tokenHdr: "x-mcp-token-val",
			want:     "x-mcp-token-val",
		},
		{
			name:     "authorization takes precedence",
			authHdr:  "Bearer auth-token",
			tokenHdr: "x-mcp-token-val",
			want:     "auth-token",
		},
		{
			name:     "no tokens",
			authHdr:  "",
			tokenHdr: "",
			want:     "",
		},
		{
			name:     "empty bearer prefix",
			authHdr:  "Bearer ",
			tokenHdr: "",
			want:     "",
		},
		{
			name:     "non-bearer auth header",
			authHdr:  "Basic dXNlcjpwYXNz",
			tokenHdr: "",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.authHdr != "" {
				req.Header.Set("Authorization", tt.authHdr)
			}
			if tt.tokenHdr != "" {
				req.Header.Set("X-MCP-Token", tt.tokenHdr)
			}

			got := extractToken(req)
			if got != tt.want {
				t.Errorf("extractToken() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAuthWithConfig_TokenMatches(t *testing.T) {
	cfg := &config.Config{
		MCP: struct {
			Enabled bool   `yaml:"enabled"`
			Path    string `yaml:"path"`
			Token   string `yaml:"token"`
		}{
			Token: "correct-token",
		},
	}

	handler := AuthWithConfig(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, ok := SessionFromContext(r.Context())
		if !ok {
			t.Fatal("expected session in context")
		}
		s, ok := sess.AuthSession.(*auth.Session)
		if !ok {
			t.Fatal("AuthSession is not *auth.Session")
		}
		if s.Username != "mcp-agent" {
			t.Errorf("expected username mcp-agent, got %q", s.Username)
		}
		if s.Role != "editor" {
			t.Errorf("expected role editor, got %q", s.Role)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer correct-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAuthWithConfig_XMCPHeaderToken(t *testing.T) {
	cfg := &config.Config{
		MCP: struct {
			Enabled bool   `yaml:"enabled"`
			Path    string `yaml:"path"`
			Token   string `yaml:"token"`
		}{
			Token: "correct-token",
		},
	}

	handler := AuthWithConfig(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, ok := SessionFromContext(r.Context())
		if !ok {
			t.Fatal("expected session in context")
		}
		s, ok := sess.AuthSession.(*auth.Session)
		if !ok {
			t.Fatal("AuthSession is not *auth.Session")
		}
		if s.Username != "mcp-agent" {
			t.Errorf("expected username mcp-agent, got %q", s.Username)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("X-MCP-Token", "correct-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAuthWithConfig_WrongToken(t *testing.T) {
	cfg := &config.Config{
		MCP: struct {
			Enabled bool   `yaml:"enabled"`
			Path    string `yaml:"path"`
			Token   string `yaml:"token"`
		}{
			Token: "correct-token",
		},
	}

	handler := AuthWithConfig(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when auth fails")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuthWithConfig_EmptyTokenProvided(t *testing.T) {
	cfg := &config.Config{
		MCP: struct {
			Enabled bool   `yaml:"enabled"`
			Path    string `yaml:"path"`
			Token   string `yaml:"token"`
		}{
			Token: "correct-token",
		},
	}

	handler := AuthWithConfig(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when token is empty")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer ")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuthWithConfig_NoTokenConfigured(t *testing.T) {
	cfg := &config.Config{}

	origGetSession := auth.GetSessionFunc
	auth.GetSessionFunc = func(r *http.Request) *auth.Session {
		return &auth.Session{
			Username: "testuser",
			Role:     "editor",
		}
	}
	defer func() { auth.GetSessionFunc = origGetSession }()

	handler := AuthWithConfig(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, ok := SessionFromContext(r.Context())
		if !ok {
			t.Fatal("expected session in context")
		}
		s, ok := sess.AuthSession.(*auth.Session)
		if !ok {
			t.Fatal("AuthSession is not *auth.Session")
		}
		if s.Username != "testuser" {
			t.Errorf("expected username testuser, got %q", s.Username)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAuthWithConfig_NoAuth(t *testing.T) {
	cfg := &config.Config{
		MCP: struct {
			Enabled bool   `yaml:"enabled"`
			Path    string `yaml:"path"`
			Token   string `yaml:"token"`
		}{
			Token: "some-token",
		},
	}

	handler := AuthWithConfig(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called without auth")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}

func TestAuthWithConfig_SessionCookieFallback(t *testing.T) {
	cfg := &config.Config{}

	origGetSession := auth.GetSessionFunc
	auth.GetSessionFunc = func(r *http.Request) *auth.Session {
		return &auth.Session{
			Username: "cookieuser",
			Role:     "viewer",
		}
	}
	defer func() { auth.GetSessionFunc = origGetSession }()

	handler := AuthWithConfig(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, ok := SessionFromContext(r.Context())
		if !ok {
			t.Fatal("expected session in context")
		}
		s, ok := sess.AuthSession.(*auth.Session)
		if !ok {
			t.Fatal("AuthSession is not *auth.Session")
		}
		if s.Username != "cookieuser" {
			t.Errorf("expected username cookieuser, got %q", s.Username)
		}
		if s.Role != "viewer" {
			t.Errorf("expected role viewer, got %q", s.Role)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAuthWithConfig_TokenTakesPrecedenceOverSession(t *testing.T) {
	cfg := &config.Config{
		MCP: struct {
			Enabled bool   `yaml:"enabled"`
			Path    string `yaml:"path"`
			Token   string `yaml:"token"`
		}{
			Token: "api-token",
		},
	}

	origGetSession := auth.GetSessionFunc
	auth.GetSessionFunc = func(r *http.Request) *auth.Session {
		return &auth.Session{
			Username: "cookieuser",
			Role:     "viewer",
		}
	}
	defer func() { auth.GetSessionFunc = origGetSession }()

	handler := AuthWithConfig(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, ok := SessionFromContext(r.Context())
		if !ok {
			t.Fatal("expected session in context")
		}
		s, ok := sess.AuthSession.(*auth.Session)
		if !ok {
			t.Fatal("AuthSession is not *auth.Session")
		}
		if s.Username != "mcp-agent" {
			t.Errorf("expected mcp-agent (token user), got %q", s.Username)
		}
		if s.Role != "editor" {
			t.Errorf("expected editor (token role), got %q", s.Role)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer api-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
