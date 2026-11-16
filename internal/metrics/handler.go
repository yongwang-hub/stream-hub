package metrics

import (
	"net/http"

	"stream-hub/internal/httpx"
)

// Handler 提供首屏快照接口：GET /api/v1/metrics。
// 实时增量走 SSE 频道 system-metrics（复用现成的 stream.Hub 订阅端点）。
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Snapshot(w http.ResponseWriter, _ *http.Request) {
	latest, ok := h.svc.Latest()
	if !ok {
		httpx.OK(w, map[string]any{"latest": nil, "history": []Snapshot{}})
		return
	}
	httpx.OK(w, map[string]any{"latest": latest, "history": h.svc.History()})
}
