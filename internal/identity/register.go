package identity

import (
	"stream-hub/internal/platform/auth"
	platformhttp "stream-hub/internal/platform/http"
)

// Register 声明用户/角色管理路由，全部需 config 域之外的 RBAC 权限码。
func (h *AdminHandler) Register(r *platformhttp.Router) {
	r.Authorize("GET /api/v1/users", auth.PermUserList, h.ListUsers)
	r.Authorize("POST /api/v1/users", auth.PermUserCreate, h.CreateUser)
	r.Authorize("PUT /api/v1/users/{id}", auth.PermUserUpdate, h.UpdateUser)
	r.Authorize("DELETE /api/v1/users/{id}", auth.PermUserDelete, h.DeleteUser)

	r.Authorize("GET /api/v1/roles", auth.PermRoleManage, h.ListRoles)
	r.Authorize("POST /api/v1/roles", auth.PermRoleManage, h.CreateRole)
	r.Authorize("PUT /api/v1/roles/{id}", auth.PermRoleManage, h.UpdateRole)
	r.Authorize("DELETE /api/v1/roles/{id}", auth.PermRoleManage, h.DeleteRole)
}
