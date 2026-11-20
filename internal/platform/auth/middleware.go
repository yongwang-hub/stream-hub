package auth

import (
	"log/slog"
	"net/http"
	"strings"

	"stream-hub/internal/httpx"
)

// Middleware 认证关卡（401）：Authorization: Bearer <token> → 验签 → Claims 进 context → 放行。
func (a *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		token, found := strings.CutPrefix(authHeader, "Bearer ")
		if !found || token == "" {
			httpx.SystemError(w, http.StatusUnauthorized, httpx.CodeSystemUnauth, "missing bearer token")
			return
		}
		claims, err := a.Verify(token)
		if err != nil {
			msg := "invalid token"
			if err == ErrTokenExpired {
				msg = "token expired"
			}
			slog.Warn("auth failed", "path", r.URL.Path, "reason", err.Error())
			httpx.SystemError(w, http.StatusUnauthorized, httpx.CodeSystemUnauth, msg)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithClaims(r.Context(), claims)))
	})
}

// RequirePermission 鉴权关卡（403）：判权限码而非角色名，权限集合每次实时查库。
func (a *Authenticator) RequirePermission(permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if a.accounts == nil {
				slog.Error("authorize requested but account store not configured")
				httpx.InternalError(w)
				return
			}
			claims := FromContext(r.Context())
			if claims == nil {
				httpx.SystemError(w, http.StatusUnauthorized, httpx.CodeSystemUnauth, "authentication required")
				return
			}
			uid, err := UIDFromSubject(claims.Subject)
			if err != nil {
				httpx.SystemError(w, http.StatusUnauthorized, httpx.CodeSystemUnauth, "malformed token subject")
				return
			}
			info, err := a.accounts.Authorize(r.Context(), uid)
			if err != nil {
				slog.Error("authorize lookup failed", "user_id", uid, "err", err)
				httpx.InternalError(w)
				return
			}
			if !info.Enabled {
				slog.Warn("request denied: account disabled", "path", r.URL.Path, "user_id", uid)
				httpx.SystemError(w, http.StatusForbidden, httpx.CodeSystemForbidden, "account disabled")
				return
			}
			// 改密即吊销旧 token：iat 早于 password_changed_at 就拒绝。
			if claims.IssuedAt != nil && !info.PasswordChangedAt.IsZero() &&
				info.PasswordChangedAt.After(claims.IssuedAt.Time) {
				httpx.SystemError(w, http.StatusUnauthorized, httpx.CodeSystemUnauth,
					"token invalidated by password change")
				return
			}
			if !info.Permissions[permission] {
				slog.Warn("permission denied",
					"path", r.URL.Path, "user_id", uid, "want_permission", permission)
				httpx.SystemError(w, http.StatusForbidden, httpx.CodeSystemForbidden,
					"permission "+permission+" required")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// MiddlewareQueryToken 给 SSE 用：除 Bearer 头外还接受 ?token=<jwt>。只验签，不查权限。
func (a *Authenticator) MiddlewareQueryToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := ""
		if header := r.Header.Get("Authorization"); strings.HasPrefix(header, "Bearer ") {
			token = strings.TrimPrefix(header, "Bearer ")
		}
		if token == "" {
			token = r.URL.Query().Get("token")
		}
		if token == "" {
			httpx.SystemError(w, http.StatusUnauthorized, httpx.CodeSystemUnauth, "missing bearer token")
			return
		}
		claims, err := a.Verify(token)
		if err != nil {
			httpx.SystemError(w, http.StatusUnauthorized, httpx.CodeSystemUnauth, "invalid token")
			return
		}
		next.ServeHTTP(w, r.WithContext(WithClaims(r.Context(), claims)))
	})
}

// CodePasswordChangeRequired 首登必须先改密的业务码（HTTP 200）。
const CodePasswordChangeRequired = 10100

// EnforcePasswordChange 强制改密关卡：MustChange=true 时挡在所有业务接口之外。
func (a *Authenticator) EnforcePasswordChange(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if claims := FromContext(r.Context()); claims != nil && claims.MustChange {
			httpx.BizError(w, CodePasswordChangeRequired, "password change required before using this API")
			return
		}
		next.ServeHTTP(w, r)
	})
}
