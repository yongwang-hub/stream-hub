package stream

import (
	"stream-hub/internal/platform/auth"
	platformhttp "stream-hub/internal/platform/http"
)

// Register 声明流相关路由：事件注入是 JSON（走 json mux），订阅是 SSE（走流式 mux）。
func (h *Handler) Register(r *platformhttp.Router) {
	r.Authorize("POST /api/v1/stream/{channel}/events", auth.PermStreamPublish, h.Publish)
	r.Stream("GET /api/v1/stream/{channel}", h.Subscribe)
}
