package metrics

import platformhttp "stream-hub/internal/platform/http"

// Register 声明指标快照路由：登录即可查看；实时增量走 stream 模块的 SSE 频道。
func (h *Handler) Register(r *platformhttp.Router) {
	r.NeedAuth("GET /api/v1/metrics", h.Snapshot)
}
