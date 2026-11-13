package user

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"stream-hub/internal/httpx"
)

// 用户模块的业务码（1xxxx 段，独立命名空间，绝不与 HTTP 数字混用）。
const (
	CodeInvalidParam  = 10001 // 参数校验不通过
	CodeUserNotFound  = 10002 // 用户不存在
	CodeDuplicateName = 10003 // 名称已存在（业务冲突）
)

// Handler 是接口层：只负责「HTTP 请求 ⇄ Service 调用」的翻译，不含业务规则。
// 它依赖 *Service（业务层），不再直接持有 Store——分层各司其职。
type Handler struct {
	svc *Service
}

// NewHandler 用给定的 Service 装配一个 Handler。
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// userRequest 是 POST/PUT 的请求体结构。json 标签决定前端传的字段名。
type userRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Create 处理 POST /users。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req userRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.BadRequest(w, "invalid request body") // body 不是合法 JSON → 框架层真实 400
		return
	}

	u, err := h.svc.Create(r.Context(), req.Name, req.Email)
	if err != nil {
		h.writeError(w, err)
		return
	}
	httpx.OK(w, u)
}

// List 处理 GET /users。
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	httpx.OK(w, h.svc.List(r.Context()))
}

// Get 处理 GET /users/{id}。
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpx.BadRequest(w, "invalid user id") // id 不是数字 → 框架层真实 400
		return
	}

	u, err := h.svc.Get(r.Context(), id)
	if err != nil {
		h.writeError(w, err)
		return
	}
	httpx.OK(w, u)
}

// Update 处理 PUT /users/{id}。
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpx.BadRequest(w, "invalid user id")
		return
	}

	var req userRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.BadRequest(w, "invalid request body")
		return
	}

	u, err := h.svc.Update(r.Context(), id, req.Name, req.Email)
	if err != nil {
		h.writeError(w, err)
		return
	}
	httpx.OK(w, u)
}

// Delete 处理 DELETE /users/{id}。
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpx.BadRequest(w, "invalid user id")
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		h.writeError(w, err)
		return
	}
	httpx.OK(w, map[string]int{"deleted": id})
}

// writeError 是这套方案的核心：把下层的错误翻译成对外的「业务码」，一律 HTTP 200。
//   - *ValidationError（业务层的参数校验）→ 用 errors.As 取出、回其用户可读的 Msg。
//   - ErrDuplicateName / ErrNotFound（数据层哨兵错误）→ 用 errors.Is 精确匹配。
//   - 都不认识 → 未知错误，不透传细节，按系统错误处理（真实 500）。
func (h *Handler) writeError(w http.ResponseWriter, err error) {
	var vErr *ValidationError
	switch {
	case errors.As(err, &vErr):
		httpx.BizError(w, CodeInvalidParam, vErr.Msg)
	case errors.Is(err, ErrDuplicateName):
		httpx.BizError(w, CodeDuplicateName, "用户名已存在")
	case errors.Is(err, ErrNotFound):
		httpx.BizError(w, CodeUserNotFound, "用户不存在")
	default:
		httpx.InternalError(w)
	}
}

// pathID 取出路径参数 id 并转成 int；转失败即请求语法错误，由调用方回 400。
func pathID(r *http.Request) (int, error) {
	return strconv.Atoi(r.PathValue("id"))
}
