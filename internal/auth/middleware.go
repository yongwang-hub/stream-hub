// 认证/鉴权中间件：把 token 校验做成「路过的关卡」，业务 handler 完全无感。
package auth

import (
	"log/slog"
	"net/http"
	"strings"

	"stream-hub/internal/httpx"
)

// Middleware 认证关卡（401 档——发生在业务逻辑之外，用真实 HTTP 状态码，
// 正是我们响应规范里预留的 90004 出场的地方）：
//
//	Authorization: Bearer <token>  →  验签  →  Claims 进 context  →  放行
//	缺头 / 格式错 / 验签失败 / 过期  →  401 信封，不再往下走
func (a *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		token, found := strings.CutPrefix(authHeader, "Bearer ") // 规范：方案名大小写敏感
		if !found || token == "" {
			httpx.SystemError(w, http.StatusUnauthorized, httpx.CodeSystemUnauth, "missing bearer token")
			return
		}

		claims, err := a.Verify(token)
		if err != nil {
			msg := "invalid token"
			if err == ErrTokenExpired {
				msg = "token expired" // 客户端能区分：重登即可 vs token 有篡改嫌疑
			}
			slog.Warn("auth failed", "path", r.URL.Path, "reason", err.Error())
			httpx.SystemError(w, http.StatusUnauthorized, httpx.CodeSystemUnauth, msg)
			return
		}

		// 验证通过：身份放进 context，下游（业务 handler / RequireRole）随取。
		next.ServeHTTP(w, r.WithContext(WithClaims(r.Context(), claims)))
	})
}

// RequirePermission 鉴权关卡（403 档）：认证过了（知道你是谁），但权限不够。
// 必须包在 Middleware 之内层，Claims 才取得到。
//
// 现代要点：判的是"权限码"而非"角色名"（handler 问"能不能 user:delete"，不是"是不是 admin"）；
// 权限集合每次实时查库——于是后台调整角色权限、停用账号，立刻对已登录用户生效。
func (a *Authenticator) RequirePermission(permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if a.accounts == nil {
				slog.Error("authorize requested but account store not configured")
				httpx.InternalError(w)
				return
			}
			claims := FromContext(r.Context())
			if claims == nil { // 到不了这（Middleware 在前），防御性
				httpx.SystemError(w, http.StatusUnauthorized, httpx.CodeSystemUnauth, "authentication required")
				return
			}
			uid, err := uidFromSubject(claims.Subject)
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
			if !info.Enabled { // 身份有效但账号已停用 → 403（可能发生在 token 签发之后，故每次复查）
				slog.Warn("request denied: account disabled", "path", r.URL.Path, "user_id", uid)
				httpx.SystemError(w, http.StatusForbidden, httpx.CodeSystemForbidden, "account disabled")
				return
			}
			// 改密即吊销旧 token：token 的 iat 早于最近一次 password_changed_at 就拒绝。
			// 不引入 token 黑名单/版本号，复用已有列，零额外存储。
			if claims.IssuedAt != nil && !info.PasswordChangedAt.IsZero() &&
				info.PasswordChangedAt.After(claims.IssuedAt.Time) {
				httpx.SystemError(w, http.StatusUnauthorized, httpx.CodeSystemUnauth,
					"token invalidated by password change")
				return
			}
			if !info.Permissions[permission] {
				slog.Warn("permission denied",
					"path", r.URL.Path, "user_id", uid, "want_permission", permission)
				// 给客户端的回答保持含糊（只说需要什么权限），细节留在日志里。
				httpx.SystemError(w, http.StatusForbidden, httpx.CodeSystemForbidden,
					"permission "+permission+" required")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// MiddlewareQueryToken 给 SSE 用：EventSource 不能自定义 Authorization 头，
// 因此除标准 Bearer 头外还接受 ?token=<jwt>。只验签，不查权限、不套强制改密关卡
// （长连接上返回 JSON 信封没有意义，交给前端按 401 处理即可）。
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

// CodePasswordChangeRequired 是"首登必须先改密"的业务码（HTTP 仍 200，属进到策略之后的结果）。
const CodePasswordChangeRequired = 10100

// EnforcePasswordChange 强制改密关卡：token 带 MustChange=true（还没改过初始密码）时，
// 把用户挡在所有业务接口之外，只留改密一条路。包在 Middleware 之内、RequirePermission 之外。
// 改密接口本身不套这道关卡，因此能顺利抵达。
func (a *Authenticator) EnforcePasswordChange(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if claims := FromContext(r.Context()); claims != nil && claims.MustChange {
			httpx.BizError(w, CodePasswordChangeRequired, "password change required before using this API")
			return
		}
		next.ServeHTTP(w, r)
	})
}
