package systemconfig

import (
	"stream-hub/internal/platform/auth"
	platformhttp "stream-hub/internal/platform/http"
)

// Register 声明系统配置路由：读公开（登录页要渲染品牌），写与上传需 config:manage。
func (h *Handler) Register(r *platformhttp.Router) {
	r.Public("GET /api/v1/system-config", h.Get)
	r.Authorize("PUT /api/v1/system-config", auth.PermConfigManage, h.Update)
	r.Authorize("POST /api/v1/system-config/upload", auth.PermConfigManage, h.Upload)
}
