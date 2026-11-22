package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/danielgtaylor/huma/v2"

	"stream-hub/internal/httpx"
	platformhttp "stream-hub/internal/platform/http"
)

// Huma DTO：字段 tag 同时驱动 OpenAPI schema 与运行期校验。
type loginInput struct {
	Body struct {
		Username string `json:"username" doc:"用户名" minLength:"1"`
		Password string `json:"password" doc:"密码" minLength:"1"`
	} `json:"body"`
}

type passwordInput struct {
	Body struct {
		OldPassword string `json:"old_password" doc:"当前密码" minLength:"1"`
		NewPassword string `json:"new_password" doc:"新密码，至少 6 位" minLength:"6"`
	} `json:"body"`
}

type tokenOutput struct {
	Body httpx.Response `json:"body"`
}

func tokenEnvelope(code int, msg string, token string, ttl int, mustChange bool) tokenOutput {
	return tokenOutput{Body: httpx.Response{
		Code: code,
		Msg:  msg,
		Data: map[string]any{
			"token": token, "expires_in": ttl, "must_change": mustChange,
		},
	}}
}

// RegisterHuma 用 code-first 声明认证接口：
// login 公开（401 是凭据错误，前端 auth:false 特判，不跳登录页）；改密需登录态。
func (a *Authenticator) RegisterHuma(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "login",
		Method:      http.MethodPost,
		Path:        "/login",
		Summary:     "登录",
		Description: "用户名密码换取 JWT；首登 must_change=true，前端需引导改密。",
		Tags:        []string{"Auth"},
		Security:    []map[string][]string{{}},
	}, func(ctx context.Context, in *loginInput) (*tokenOutput, error) {
		if a.accounts == nil {
			slog.Default().LogAttrs(ctx, slog.LevelError, "login requested but account store not configured")
			return nil, huma.Error500InternalServerError("account store not configured")
		}

		acct, err := a.accounts.FindForLogin(ctx, in.Body.Username)
		switch {
		case errors.Is(err, ErrAccountNotFound):
			_ = CheckPassword(dummyHash, in.Body.Password)
			slog.Default().LogAttrs(ctx, slog.LevelWarn, "login failed",
				slog.String("reason", "unknown_user"), slog.String("username", in.Body.Username))
			return nil, huma.Error401Unauthorized("invalid credentials")
		case err != nil:
			slog.Default().LogAttrs(ctx, slog.LevelError, "login lookup error", slog.Any("err", err))
			return nil, huma.Error500InternalServerError("login lookup failed")
		}
		if CheckPassword(acct.PasswordHash, in.Body.Password) != nil {
			slog.Default().LogAttrs(ctx, slog.LevelWarn, "login failed",
				slog.String("reason", "bad_password"), slog.String("username", in.Body.Username))
			return nil, huma.Error401Unauthorized("invalid credentials")
		}
		if !acct.Enabled {
			return nil, huma.Error403Forbidden("account disabled")
		}

		mustChange := !acct.PasswordChanged
		token, err := a.Issue(strconv.Itoa(acct.ID), mustChange)
		if err != nil {
			return nil, huma.Error500InternalServerError("issue token failed")
		}
		slog.Default().LogAttrs(ctx, slog.LevelInfo, "login success",
			slog.Int("user_id", acct.ID), slog.Bool("must_change_password", mustChange))

		code := httpx.CodeOK
		if mustChange {
			code = CodePasswordChangeRequired
		}
		out := tokenEnvelope(code, "ok", token, int(a.TokenTTL().Seconds()), mustChange)
		return &out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "change-password",
		Method:      http.MethodPut,
		Path:        "/me/password",
		Summary:     "修改当前用户密码",
		Description: "改密后当前 token 立即失效，响应返回新 token。",
		Tags:        []string{"Auth"},
		Security:    platformhttp.BearerSecurity(),
	}, func(ctx context.Context, in *passwordInput) (*tokenOutput, error) {
		claims := FromContext(ctx)
		if claims == nil {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		uid, err := UIDFromSubject(claims.Subject)
		if err != nil {
			return nil, huma.Error401Unauthorized("malformed token subject")
		}

		if err := a.accounts.ChangePassword(ctx, uid, in.Body.OldPassword, in.Body.NewPassword); err != nil {
			switch {
			case errors.Is(err, ErrPasswordMismatch):
				return nil, huma.Error401Unauthorized("old password incorrect")
			case errors.Is(err, ErrAccountNotFound):
				return nil, huma.Error401Unauthorized("account not found")
			default:
				slog.Default().LogAttrs(ctx, slog.LevelError, "change password failed",
					slog.Int("user_id", uid), slog.Any("err", err))
				return nil, huma.Error500InternalServerError("change password failed")
			}
		}

		token, err := a.Issue(claims.Subject, false)
		if err != nil {
			return nil, huma.Error500InternalServerError("issue token failed")
		}
		slog.Default().LogAttrs(ctx, slog.LevelInfo, "password changed", slog.Int("user_id", uid))
		out := tokenEnvelope(httpx.CodeOK, "ok", token, int(a.TokenTTL().Seconds()), false)
		return &out, nil
	})
}
