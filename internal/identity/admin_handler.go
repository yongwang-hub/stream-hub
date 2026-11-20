// 管理端 HTTP 接口：用户 / 角色的增删改查。
// 认证、强制改密、权限判定由 platform/http 注册器在外层完成，这里只管请求体 ⇄ Repo。
package identity

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"stream-hub/internal/httpx"
	"stream-hub/internal/platform/auth"
)

// 管理端业务码（102xx 段；101xx 归认证关卡，见 platform/auth）。
const (
	CodeAdminInvalid   = 10201
	CodeAdminNotFound  = 10202
	CodeAdminDuplicate = 10203
	CodeAdminForbidden = 10204
)

type AdminHandler struct {
	repo *AdminRepo
}

func NewAdminHandler(repo *AdminRepo) *AdminHandler {
	return &AdminHandler{repo: repo}
}

// ---- 用户 ----

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.repo.ListUsers(r.Context())
	if err != nil {
		h.writeError(w, err)
		return
	}
	httpx.OK(w, users)
}

func (h *AdminHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeBody[AdminUserInput](w, r)
	if !ok {
		return
	}
	u, err := h.repo.CreateUser(r.Context(), in)
	if err != nil {
		h.writeError(w, err)
		return
	}
	httpx.OK(w, u)
}

func (h *AdminHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathIDOrBadRequest(w, r)
	if !ok {
		return
	}
	in, ok := decodeBody[AdminUserInput](w, r)
	if !ok {
		return
	}
	u, err := h.repo.UpdateUser(r.Context(), id, in)
	if err != nil {
		h.writeError(w, err)
		return
	}
	httpx.OK(w, u)
}

func (h *AdminHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathIDOrBadRequest(w, r)
	if !ok {
		return
	}
	// 不允许删掉自己：否则当前 token 立刻失效，且可能把系统锁在没有管理员的状态。
	if claims := auth.FromContext(r.Context()); claims != nil {
		if uid, err := auth.UIDFromSubject(claims.Subject); err == nil && uid == id {
			httpx.BizError(w, CodeAdminForbidden, "不能删除当前登录账号")
			return
		}
	}
	if err := h.repo.DeleteUser(r.Context(), id); err != nil {
		h.writeError(w, err)
		return
	}
	httpx.OK(w, map[string]int{"deleted": id})
}

// ---- 角色 ----

func (h *AdminHandler) ListRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := h.repo.ListRoles(r.Context())
	if err != nil {
		h.writeError(w, err)
		return
	}
	httpx.OK(w, roles)
}

func (h *AdminHandler) CreateRole(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeBody[AdminRoleInput](w, r)
	if !ok {
		return
	}
	role, err := h.repo.CreateRole(r.Context(), in)
	if err != nil {
		h.writeError(w, err)
		return
	}
	httpx.OK(w, role)
}

func (h *AdminHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	id, ok := pathIDOrBadRequest(w, r)
	if !ok {
		return
	}
	in, ok := decodeBody[AdminRoleInput](w, r)
	if !ok {
		return
	}
	role, err := h.repo.UpdateRole(r.Context(), id, in)
	if err != nil {
		h.writeError(w, err)
		return
	}
	httpx.OK(w, role)
}

func (h *AdminHandler) DeleteRole(w http.ResponseWriter, r *http.Request) {
	id, ok := pathIDOrBadRequest(w, r)
	if !ok {
		return
	}
	if err := h.repo.DeleteRole(r.Context(), id); err != nil {
		h.writeError(w, err)
		return
	}
	httpx.OK(w, map[string]int{"deleted": id})
}

// ---- 公共小工具 ----

func decodeBody[T any](w http.ResponseWriter, r *http.Request) (T, bool) {
	var v T
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		httpx.BadRequest(w, "invalid request body")
		return v, false
	}
	return v, true
}

func pathIDOrBadRequest(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		httpx.BadRequest(w, "invalid id")
		return 0, false
	}
	return id, true
}

// writeError 把数据层哨兵错误翻成业务码；未知错误只记日志、对外统一 500 文案。
func (h *AdminHandler) writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrAdminInvalid):
		httpx.BizError(w, CodeAdminInvalid, cleanErr(err))
	case errors.Is(err, ErrAdminNotFound):
		httpx.BizError(w, CodeAdminNotFound, "记录不存在")
	case errors.Is(err, ErrAdminDuplicate):
		httpx.BizError(w, CodeAdminDuplicate, cleanErr(err))
	default:
		slog.Error("admin operation failed", "err", err)
		httpx.InternalError(w)
	}
}

// cleanErr 去掉哨兵错误前缀，只留给人看的原因（如 "admin: invalid input: role 9 does not exist"）。
func cleanErr(err error) string {
	msg := err.Error()
	if i := strings.LastIndex(msg, ": "); i >= 0 {
		return msg[i+2:]
	}
	return msg
}
