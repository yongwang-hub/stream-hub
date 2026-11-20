package media

import (
	"stream-hub/internal/platform/auth"
	platformhttp "stream-hub/internal/platform/http"
)

// Register 声明频道路由：读只需登录态，写需要 channel:manage。
func (h *Handler) Register(r *platformhttp.Router) {
	r.NeedAuth("GET /api/v1/channels", h.List)
	r.NeedAuth("GET /api/v1/channels/{id}", h.Get)
	r.Authorize("POST /api/v1/channels", auth.PermChannelManage, h.Create)
	r.Authorize("PUT /api/v1/channels/{id}", auth.PermChannelManage, h.Update)
	r.Authorize("DELETE /api/v1/channels/{id}", auth.PermChannelManage, h.Delete)
}
