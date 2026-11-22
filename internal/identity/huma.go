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
// 输出用 httpx.Envelope[T]（T 具体），让 data 结构进入 OpenAPI 供前端生成类型。

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

// AdminDeleteResult 是删除接口的统一载荷。
type AdminDeleteResult struct {
	Deleted int `json:"deleted"`
}

type usersOutput struct {
	Body httpx.Envelope[[]AdminUser] `json:"body"`
}
type userOutput struct {
	Body httpx.Envelope[AdminUser] `json:"body"`
}
type rolesOutput struct {
	Body httpx.Envelope[[]AdminRole] `json:"body"`
}
type roleOutput struct {
	Body httpx.Envelope[AdminRole] `json:"body"`
}
type deleteOutput struct {
	Body httpx.Envelope[AdminDeleteResult] `json:"body"`
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
	}, func(ctx context.Context, _ *listUsersInput) (*usersOutput, error) {
		users, err := h.repo.ListUsers(ctx)
		if err != nil {
			return nil, adminHumaErr(ctx, err)
		}
		return &usersOutput{Body: httpx.Envelope[[]AdminUser]{Code: httpx.CodeOK, Msg: "ok", Data: users}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-user", Method: http.MethodPost, Path: "/api/v1/users",
		Summary: "新建用户", Tags: userTag, Security: platformhttp.BearerSecurity(),
		Metadata: map[string]any{"permission": auth.PermUserCreate},
	}, func(ctx context.Context, in *createUserInput) (*userOutput, error) {
		u, err := h.repo.CreateUser(ctx, in.Body)
		if err != nil {
			return nil, adminHumaErr(ctx, err)
		}
		return &userOutput{Body: httpx.Envelope[AdminUser]{Code: httpx.CodeOK, Msg: "ok", Data: u}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-user", Method: http.MethodPut, Path: "/api/v1/users/{id}",
		Summary: "修改用户", Tags: userTag, Security: platformhttp.BearerSecurity(),
		Metadata: map[string]any{"permission": auth.PermUserUpdate},
	}, func(ctx context.Context, in *updateUserInput) (*userOutput, error) {
		u, err := h.repo.UpdateUser(ctx, in.ID, in.Body)
		if err != nil {
			return nil, adminHumaErr(ctx, err)
		}
		return &userOutput{Body: httpx.Envelope[AdminUser]{Code: httpx.CodeOK, Msg: "ok", Data: u}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-user", Method: http.MethodDelete, Path: "/api/v1/users/{id}",
		Summary: "删除用户", Tags: userTag, Security: platformhttp.BearerSecurity(),
		Metadata: map[string]any{"permission": auth.PermUserDelete},
	}, func(ctx context.Context, in *deleteUserInput) (*deleteOutput, error) {
		if err := h.repo.DeleteUser(ctx, in.ID); err != nil {
			return nil, adminHumaErr(ctx, err)
		}
		return &deleteOutput{Body: httpx.Envelope[AdminDeleteResult]{
			Code: httpx.CodeOK, Msg: "ok", Data: AdminDeleteResult{Deleted: in.ID},
		}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-roles", Method: http.MethodGet, Path: "/api/v1/roles",
		Summary: "角色列表", Tags: roleTag, Security: platformhttp.BearerSecurity(),
		Metadata: map[string]any{"permission": auth.PermRoleManage},
	}, func(ctx context.Context, _ *listRolesInput) (*rolesOutput, error) {
		roles, err := h.repo.ListRoles(ctx)
		if err != nil {
			return nil, adminHumaErr(ctx, err)
		}
		return &rolesOutput{Body: httpx.Envelope[[]AdminRole]{Code: httpx.CodeOK, Msg: "ok", Data: roles}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-role", Method: http.MethodPost, Path: "/api/v1/roles",
		Summary: "新建角色", Tags: roleTag, Security: platformhttp.BearerSecurity(),
		Metadata: map[string]any{"permission": auth.PermRoleManage},
	}, func(ctx context.Context, in *createRoleInput) (*roleOutput, error) {
		r, err := h.repo.CreateRole(ctx, in.Body)
		if err != nil {
			return nil, adminHumaErr(ctx, err)
		}
		return &roleOutput{Body: httpx.Envelope[AdminRole]{Code: httpx.CodeOK, Msg: "ok", Data: r}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-role", Method: http.MethodPut, Path: "/api/v1/roles/{id}",
		Summary: "修改角色", Tags: roleTag, Security: platformhttp.BearerSecurity(),
		Metadata: map[string]any{"permission": auth.PermRoleManage},
	}, func(ctx context.Context, in *updateRoleInput) (*roleOutput, error) {
		r, err := h.repo.UpdateRole(ctx, in.ID, in.Body)
		if err != nil {
			return nil, adminHumaErr(ctx, err)
		}
		return &roleOutput{Body: httpx.Envelope[AdminRole]{Code: httpx.CodeOK, Msg: "ok", Data: r}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-role", Method: http.MethodDelete, Path: "/api/v1/roles/{id}",
		Summary: "删除角色", Tags: roleTag, Security: platformhttp.BearerSecurity(),
		Metadata: map[string]any{"permission": auth.PermRoleManage},
	}, func(ctx context.Context, in *deleteRoleInput) (*deleteOutput, error) {
		if err := h.repo.DeleteRole(ctx, in.ID); err != nil {
			return nil, adminHumaErr(ctx, err)
		}
		return &deleteOutput{Body: httpx.Envelope[AdminDeleteResult]{
			Code: httpx.CodeOK, Msg: "ok", Data: AdminDeleteResult{Deleted: in.ID},
		}}, nil
	})
}
