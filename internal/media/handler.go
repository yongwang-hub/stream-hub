package media

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"stream-hub/internal/httpx"
)

// 频道模块业务码（2xxxx 段，与 httpx 的 9xxxx 系统码分开）。
const (
	CodeChannelInvalidParam = 20001
	CodeChannelNotFound     = 20002
	CodeDuplicateSlug       = 20003
)

// Handler 只做 HTTP ⇄ Service 的翻译。
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// channelRequest 同时用于旧 handler 与 Huma DTO。
// omitempty 让 Huma 把字段视为可选（PUT 支持局部更新），不影响 JSON 解码行为。
type channelRequest struct {
	Slug      string `json:"slug,omitempty"`
	Name      string `json:"name,omitempty"`
	SourceURL string `json:"source_url,omitempty"`
	Status    string `json:"status,omitempty"`
}

// List 处理 GET /api/v1/channels。
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.List(r.Context())
	if err != nil {
		httpx.InternalError(w)
		return
	}
	httpx.OK(w, list)
}

// Create 处理 POST /api/v1/channels。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req channelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.BadRequest(w, "invalid request body")
		return
	}
	c, err := h.svc.Create(r.Context(), req.Slug, req.Name, req.SourceURL)
	if err != nil {
		h.writeError(w, err)
		return
	}
	httpx.OK(w, c)
}

// Get 处理 GET /api/v1/channels/{id}。
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpx.BadRequest(w, "invalid channel id")
		return
	}
	c, err := h.svc.Get(r.Context(), id)
	if err != nil {
		h.writeError(w, err)
		return
	}
	httpx.OK(w, c)
}

// Update 处理 PUT /api/v1/channels/{id}（局部更新，空字段保留原值）。
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpx.BadRequest(w, "invalid channel id")
		return
	}
	var req channelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.BadRequest(w, "invalid request body")
		return
	}
	c, err := h.svc.Update(r.Context(), id, req.Name, req.SourceURL, req.Status)
	if err != nil {
		h.writeError(w, err)
		return
	}
	httpx.OK(w, c)
}

// Delete 处理 DELETE /api/v1/channels/{id}。
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpx.BadRequest(w, "invalid channel id")
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		h.writeError(w, err)
		return
	}
	httpx.OK(w, map[string]int64{"deleted": id})
}

func (h *Handler) writeError(w http.ResponseWriter, err error) {
	var v *ValidationError
	switch {
	case errors.As(err, &v):
		httpx.BizError(w, CodeChannelInvalidParam, v.Msg)
	case errors.Is(err, ErrNotFound):
		httpx.BizError(w, CodeChannelNotFound, "频道不存在")
	case errors.Is(err, ErrDuplicateSlug):
		httpx.BizError(w, CodeDuplicateSlug, "slug 已存在")
	default:
		httpx.InternalError(w)
	}
}

func pathID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}
