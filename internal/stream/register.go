package stream

import platformhttp "stream-hub/internal/platform/http"

// Register 只声明 SSE 订阅：它必须走流式 mux，不能被 Huma/JSONErrors 缓冲。
// 事件注入是普通 JSON 接口，已迁到 Huma（见 huma.go 的 RegisterHuma）。
func (h *Handler) Register(r *platformhttp.Router) {
	r.Stream("GET /api/v1/stream/{channel}", h.Subscribe)
}
