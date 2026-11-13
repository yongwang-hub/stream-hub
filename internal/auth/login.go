// 登录 & 改密接口：真实用户体系版。
//
// 与上一版固定 demoUsers 的区别：这里查 sys_users 表 + 用 bcrypt 校验哈希，
// token 里只放"用户 ID"（不再放角色/权限），鉴权每次实时查库。
// 并落实"首登强制改密"：初始密码固定为 admin，登录后 token 带 MustChange=true，
// 由 EnforcePasswordChange 关卡把用户挡在改密接口之外，直到改过密码。
package auth

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"stream-hub/internal/httpx"
)

// dummyHash 是"用户名不存在"时也走一遍 bcrypt 比对用的假哈希——
// 让"用户不存在"与"密码错"耗时一致，杜绝通过响应时间枚举有效用户名。
// 固定一个合法 bcrypt 串即可（内容无所谓，永远匹配不上真实输入）。
const dummyHash = "$2a$10$0000000000000000000000000000000000000000000000000000"

// Login 处理 POST /login：{"username":"admin","password":"admin"} → {"token":"...","must_change":true}。
func (a *Authenticator) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.BadRequest(w, "invalid request body") // 框架层 400，和 CRUD 那边同档
		return
	}
	if a.accounts == nil {
		slog.Error("login requested but account store not configured")
		httpx.InternalError(w)
		return
	}

	acct, err := a.accounts.FindForLogin(r.Context(), req.Username)
	switch {
	case errors.Is(err, ErrAccountNotFound):
		// 账号不存在：仍做一次等长假比对，再回与"密码错"完全相同的含糊 401。
		_ = checkPassword(dummyHash, req.Password)
		slog.Warn("login failed", "reason", "unknown user", "username", req.Username)
		httpx.SystemError(w, http.StatusUnauthorized, httpx.CodeSystemUnauth, "invalid credentials")
		return
	case err != nil:
		slog.Error("login lookup error", "err", err)
		httpx.InternalError(w)
		return
	}

	// bcrypt 内部即常量时间，无需再手动 subtle 比对（这也是换库的省心之处）。
	if checkPassword(acct.PasswordHash, req.Password) != nil {
		slog.Warn("login failed", "reason", "bad password", "username", req.Username)
		httpx.SystemError(w, http.StatusUnauthorized, httpx.CodeSystemUnauth, "invalid credentials")
		return
	}
	// 认证通过，但账号被停用 → 403（发生在业务之外，但不是"身份不明"而是"身份有效却被封"）。
	if !acct.Enabled {
		slog.Warn("login blocked: account disabled", "username", req.Username)
		httpx.SystemError(w, http.StatusForbidden, httpx.CodeSystemForbidden, "account disabled")
		return
	}

	mustChange := !acct.PasswordChanged
	token, err := a.Issue(strconv.Itoa(acct.ID), mustChange)
	if err != nil { // 序列化/随机数坏了才会到这，属系统错误
		httpx.InternalError(w)
		return
	}
	slog.Info("login success", "user_id", acct.ID, "must_change_password", mustChange)
	httpx.OK(w, map[string]any{
		"token":       token,
		"expires_in":  int(a.ttl / time.Second), // 告诉客户端多少秒后过期（比给死时间点友好）
		"must_change": mustChange,               // 前端据此立即跳转到改密页
	})
}

// ChangePassword 处理 PUT /me/password：{"old_password":"admin","new_password":"..."}。
// 需登录（带 token），但不需任何权限——改自己的密码天经地义，也因此能穿过强制改密关卡。
func (a *Authenticator) ChangePassword(w http.ResponseWriter, r *http.Request) {
	claims := FromContext(r.Context())
	if claims == nil { // 理论到不了这（已过 Middleware），防御性兜底
		httpx.SystemError(w, http.StatusUnauthorized, httpx.CodeSystemUnauth, "authentication required")
		return
	}
	uid, err := uidFromSubject(claims.Subject)
	if err != nil {
		httpx.SystemError(w, http.StatusUnauthorized, httpx.CodeSystemUnauth, "malformed token subject")
		return
	}

	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.BadRequest(w, "invalid request body")
		return
	}
	// 新密码最简策略：长度 ≥6（demo 从宽；真实项目在这里接强度校验）。
	if len(req.NewPassword) < 6 {
		httpx.BadRequest(w, "new password must be at least 6 characters")
		return
	}

	err = a.accounts.ChangePassword(r.Context(), uid, req.OldPassword, req.NewPassword)
	switch {
	case errors.Is(err, ErrPasswordMismatch):
		httpx.SystemError(w, http.StatusUnauthorized, httpx.CodeSystemUnauth, "old password incorrect")
		return
	case errors.Is(err, ErrAccountNotFound):
		httpx.SystemError(w, http.StatusUnauthorized, httpx.CodeSystemUnauth, "account not found")
		return
	case err != nil:
		slog.Error("change password failed", "user_id", uid, "err", err)
		httpx.InternalError(w)
		return
	}

	// 改密成功 → 重签一个 MustChange=false 的 token，客户端换掉旧的即可继续正常使用。
	token, err := a.Issue(claims.Subject, false)
	if err != nil {
		httpx.InternalError(w)
		return
	}
	slog.Info("password changed", "user_id", uid)
	httpx.OK(w, map[string]any{
		"token":      token,
		"expires_in": int(a.ttl / time.Second),
	})
}
