package mcp

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"
	"wiki-go/internal/auth"
	"wiki-go/internal/config"
)

// AuthWithConfig returns middleware that authenticates requests using either
// the configured API token or the standard session cookie.
//
// If cfg.MCP.Token is set and the request provides a matching token via
// "Authorization: Bearer <token>" or "X-MCP-Token", a synthetic session
// is created with editor role. If no token is configured, the middleware
// falls back to the session cookie (auth.GetSession).
//
// The authenticated session is stored in the request context and retrieved
// via SessionFromContext.
func AuthWithConfig(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var sess *auth.Session

			// Try API token first
			if cfg.MCP.Token != "" {
				if token := extractToken(r); token != "" {
					if subtle.ConstantTimeCompare([]byte(token), []byte(cfg.MCP.Token)) == 1 {
						sess = &auth.Session{
							Username: "mcp-agent",
							Role:     "editor",
						}
					}
				}
			}

			// Fall back to session cookie
			if sess == nil {
				sess = auth.GetSessionFunc(r)
			}

			// Reject if no valid session
			if sess == nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"jsonrpc":"2.0","error":{"code":-1,"message":"Unauthorized"}}`))
				return
			}

			// Store session in context
			entry := &sessionEntry{
				AuthSession: sess,
			}
			ctx := context.WithValue(r.Context(), sessionKey{}, entry)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// extractToken extracts the bearer token from the request
func extractToken(r *http.Request) string {
	// Check Authorization header
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}

	// Check X-MCP-Token header
	if token := r.Header.Get("X-MCP-Token"); token != "" {
		return token
	}

	return ""
}
