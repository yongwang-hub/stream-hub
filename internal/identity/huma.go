package identity

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"stream-hub/internal/httpx"
	"stream-hub/internal/platform/auth"
	platformhttp "stream-hub/internal/platform/http"
)

// Huma DTO：Input/Output 既是 OpenAPI schema 也是校验来源。
type envelope struct {
	Body httpx.Response `json:"body"`
}

type listUsersInput struct{}
type listRolesInput struct{}

type createUserInput struct {
	Body AdminUserInput `json:"body"`
}
type updateUserInput struct {
	ID   int            `path:"id" doc:"用户 ID" minimum:"1"`
	Body AdminUserInput `json:"body"`
}
type deleteUserInput struct {
	ID int `path:"id" doc:"用户 ID" minimum:"1"`
}

type createRoleInput struct {
	Body AdminRoleInput `json:"body"`
}
type updateRoleInput struct {
	ID   int            `path:"id" doc:"角色 ID" minimum:"1"`
	Body AdminRoleInput `json:"body"`
}
type deleteRoleInput struct {
	ID int `path:"id" doc:"角色 ID" minimum:"1"`
}

func okEnvelope(data any) envelope {
	return envelope{Body: httpx.Response{Code: httpx.CodeOK, Msg: "ok", Data: data}}
}

// adminHumaErr 把数据层哨兵错误翻成对应的 problem+json，未知错误统一 500。
func adminHumaErr(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, ErrAdminInvalid):
		return huma.Error400BadRequest(cleanErr(err))
	case errors.Is(err, ErrAdminNotFound):
		return huma.Error404NotFound("记录不存在")
	case errors.Is(err, ErrAdminDuplicate):
		return huma.Error409Conflict(cleanErr(err))
	default:
		slog.Default().LogAttrs(ctx, slog.LevelError, "admin operation failed", slog.Any("err", err))
		return huma.Error500InternalServerError("internal error")
	}
}

// RegisterHuma 用 code-first 声明用户/角色管理接口。
func (h *AdminHandler) RegisterHuma(api huma.API) {
	userTag := []string{"Users"}
	roleTag := []string{"Roles"}

	huma.Register(api, huma.Operation{
		OperationID: "list-users", Method: http.MethodGet, Path: "/api/v1/users",
		Summary: "用户列表", Tags: userTag, Security: platformhttp.BearerSecurity(),
		Metadata: map[string]any{"permission": auth.PermUserList},
	}, func(ctx context.Context, _ *listUsersInput) (*envelope, error) {
		users, err := h.repo.ListUsers(ctx)
		if err != nil {
			return nil, adminHumaErr(ctx, err)
		}
		out := okEnvelope(users)
		return &out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-user", Method: http.MethodPost, Path: "/api/v1/users",
		Summary: "新建用户", Tags: userTag, Security: platformhttp.BearerSecurity(),
		Metadata: map[string]any{"permission": auth.PermUserCreate},
	}, func(ctx context.Context, in *createUserInput) (*envelope, error) {
		u, err := h.repo.CreateUser(ctx, in.Body)
		if err != nil {
			return nil, adminHumaErr(ctx, err)
		}
		out := okEnvelope(u)
		return &out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-user", Method: http.MethodPut, Path: "/api/v1/users/{id}",
		Summary: "修改用户", Tags: userTag, Security: platformhttp.BearerSecurity(),
		Metadata: map[string]any{"permission": auth.PermUserUpdate},
	}, func(ctx context.Context, in *updateUserInput) (*envelope, error) {
		u, err := h.repo.UpdateUser(ctx, in.ID, in.Body)
		if err != nil {
			return nil, adminHumaErr(ctx, err)
		}
		out := okEnvelope(u)
		return &out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-user", Method: http.MethodDelete, Path: "/api/v1/users/{id}",
		Summary: "删除用户", Tags: userTag, Security: platformhttp.BearerSecurity(),
		Metadata: map[string]any{"permission": auth.PermUserDelete},
	}, func(ctx context.Context, in *deleteUserInput) (*envelope, error) {
		if err := h.repo.DeleteUser(ctx, in.ID); err != nil {
			return nil, adminHumaErr(ctx, err)
		}
		out := okEnvelope(map[string]int{"deleted": in.ID})
		return &out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-roles", Method: http.MethodGet, Path: "/api/v1/roles",
		Summary: "角色列表", Tags: roleTag, Security: platformhttp.BearerSecurity(),
		Metadata: map[string]any{"permission": auth.PermRoleManage},
	}, func(ctx context.Context, _ *listRolesInput) (*envelope, error) {
		roles, err := h.repo.ListRoles(ctx)
		if err != nil {
			return nil, adminHumaErr(ctx, err)
		}
		out := okEnvelope(roles)
		return &out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-role", Method: http.MethodPost, Path: "/api/v1/roles",
		Summary: "新建角色", Tags: roleTag, Security: platformhttp.BearerSecurity(),
		Metadata: map[string]any{"permission": auth.PermRoleManage},
	}, func(ctx context.Context, in *createRoleInput) (*envelope, error) {
		r, err := h.repo.CreateRole(ctx, in.Body)
		if err != nil {
			return nil, adminHumaErr(ctx, err)
		}
		out := okEnvelope(r)
		return &out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-role", Method: http.MethodPut, Path: "/api/v1/roles/{id}",
		Summary: "修改角色", Tags: roleTag, Security: platformhttp.BearerSecurity(),
		Metadata: map[string]any{"permission": auth.PermRoleManage},
	}, func(ctx context.Context, in *updateRoleInput) (*envelope, error) {
		r, err := h.repo.UpdateRole(ctx, in.ID, in.Body)
		if err != nil {
			return nil, adminHumaErr(ctx, err)
		}
		out := okEnvelope(r)
		return &out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-role", Method: http.MethodDelete, Path: "/api/v1/roles/{id}",
		Summary: "删除角色", Tags: roleTag, Security: platformhttp.BearerSecurity(),
		Metadata: map[string]any{"permission": auth.PermRoleManage},
	}, func(ctx context.Context, in *deleteRoleInput) (*envelope, error) {
		if err := h.repo.DeleteRole(ctx, in.ID); err != nil {
			return nil, adminHumaErr(ctx, err)
		}
		out := okEnvelope(map[string]int{"deleted": in.ID})
		return &out, nil
	})
}
