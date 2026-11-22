package auth

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"stream-hub/internal/httpx"
)

// dummyHash 用于"用户不存在"时也走一遍 bcrypt 比对，杜绝响应时间枚举用户名。
const dummyHash = "$2a$10$0000000000000000000000000000000000000000000000000000"

// Login 处理 POST /login：{username,password} → {token,expires_in,must_change}。
func (a *Authenticator) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.BadRequest(w, "invalid request body")
		return
	}
	if a.accounts == nil {
		slog.Default().LogAttrs(r.Context(), slog.LevelError, "login requested but account store not configured")
		httpx.InternalError(w)
		return
	}

	acct, err := a.accounts.FindForLogin(r.Context(), req.Username)
	switch {
	case errors.Is(err, ErrAccountNotFound):
		_ = CheckPassword(dummyHash, req.Password) // 等长假比对
		slog.Default().LogAttrs(r.Context(), slog.LevelWarn, "login failed",
			slog.String("reason", "unknown_user"), slog.String("username", req.Username))
		httpx.SystemError(w, http.StatusUnauthorized, httpx.CodeSystemUnauth, "invalid credentials")
		return
	case err != nil:
		slog.Default().LogAttrs(r.Context(), slog.LevelError, "login lookup error", slog.Any("err", err))
		httpx.InternalError(w)
		return
	}

	if CheckPassword(acct.PasswordHash, req.Password) != nil {
		slog.Default().LogAttrs(r.Context(), slog.LevelWarn, "login failed",
			slog.String("reason", "bad_password"), slog.String("username", req.Username))
		httpx.SystemError(w, http.StatusUnauthorized, httpx.CodeSystemUnauth, "invalid credentials")
		return
	}
	if !acct.Enabled {
		slog.Default().LogAttrs(r.Context(), slog.LevelWarn, "login blocked: account disabled",
			slog.String("username", req.Username))
		httpx.SystemError(w, http.StatusForbidden, httpx.CodeSystemForbidden, "account disabled")
		return
	}

	mustChange := !acct.PasswordChanged
	token, err := a.Issue(strconv.Itoa(acct.ID), mustChange)
	if err != nil {
		httpx.InternalError(w)
		return
	}
	slog.Default().LogAttrs(r.Context(), slog.LevelInfo, "login success",
		slog.Int("user_id", acct.ID), slog.Bool("must_change_password", mustChange))
	httpx.OK(w, map[string]any{
		"token":       token,
		"expires_in":  int(a.TokenTTL().Seconds()),
		"must_change": mustChange,
	})
}

// ChangePassword 处理 PUT /me/password：改自己的密码，成功后重签 MustChange=false 的 token。
func (a *Authenticator) ChangePassword(w http.ResponseWriter, r *http.Request) {
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

	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.BadRequest(w, "invalid request body")
		return
	}
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
		slog.Default().LogAttrs(r.Context(), slog.LevelError, "change password failed",
			slog.Int("user_id", uid), slog.Any("err", err))
		httpx.InternalError(w)
		return
	}

	token, err := a.Issue(claims.Subject, false)
	if err != nil {
		httpx.InternalError(w)
		return
	}
	slog.Default().LogAttrs(r.Context(), slog.LevelInfo, "password changed", slog.Int("user_id", uid))
	httpx.OK(w, map[string]any{
		"token":      token,
		"expires_in": int(a.TokenTTL().Seconds()),
	})
}
