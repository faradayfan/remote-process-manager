package server

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/faradayfan/remote-process-manager/internal/config"
)

type Role string

const (
	RoleAdmin Role = "admin"
	RoleRead  Role = "read"
)

type ctxKey string

const authCtxKey ctxKey = "auth"

type AuthInfo struct {
	KeyName string
	Roles   []string
}

type AuthMiddleware struct {
	keys                 map[string]*authenticatedKey
	allowUnauthenticated bool
}

type authenticatedKey struct {
	name  string
	roles map[Role]bool
}

func NewAuthMiddleware(keys []config.APIKey, allowUnauth bool) *AuthMiddleware {
	m := &AuthMiddleware{
		keys:                 make(map[string]*authenticatedKey),
		allowUnauthenticated: allowUnauth,
	}

	for _, k := range keys {
		roles := make(map[Role]bool)
		for _, r := range k.Roles {
			roles[Role(r)] = true
		}
		m.keys[k.Key] = &authenticatedKey{name: k.Name, roles: roles}
	}

	return m
}

// Wrap returns middleware that enforces authentication and role requirements.
func (a *AuthMiddleware) Wrap(required Role, next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.allowUnauthenticated {
			next.ServeHTTP(w, r)
			return
		}

		token := extractBearerToken(r)
		if token == "" {
			writeErr(w, http.StatusUnauthorized, "missing or invalid authorization header")
			return
		}

		key := a.validateKey(token)
		if key == nil {
			writeErr(w, http.StatusUnauthorized, "invalid api key")
			return
		}

		// Admin role has access to everything
		if !key.roles[required] && !key.roles[RoleAdmin] {
			writeErr(w, http.StatusForbidden, "insufficient permissions")
			return
		}

		// Add auth info to context for logging/auditing
		ctx := context.WithValue(r.Context(), authCtxKey, &AuthInfo{
			KeyName: key.name,
			Roles:   rolesToStrings(key.roles),
		})

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// WrapNoAuth wraps a handler without authentication (e.g., health check).
func (a *AuthMiddleware) WrapNoAuth(next http.HandlerFunc) http.Handler {
	return next
}

func (a *AuthMiddleware) validateKey(token string) *authenticatedKey {
	for k, v := range a.keys {
		if subtle.ConstantTimeCompare([]byte(k), []byte(token)) == 1 {
			return v
		}
	}
	return nil
}

func extractBearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}

	// Support "Bearer <token>" format
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 {
		return ""
	}

	if !strings.EqualFold(parts[0], "bearer") {
		return ""
	}

	return strings.TrimSpace(parts[1])
}

func rolesToStrings(roles map[Role]bool) []string {
	out := make([]string, 0, len(roles))
	for r := range roles {
		out = append(out, string(r))
	}
	return out
}

// GetAuthInfo retrieves auth info from context (for logging/auditing).
func GetAuthInfo(ctx context.Context) *AuthInfo {
	v := ctx.Value(authCtxKey)
	if v == nil {
		return nil
	}
	return v.(*AuthInfo)
}
