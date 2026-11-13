package stream

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"stream-hub/internal/httpx"
)

// Handler 暴露 SSE 订阅与事件发布的 HTTP 接口。
type Handler struct {
	hub *Hub
}

func NewHandler(hub *Hub) *Handler { return &Handler{hub: hub} }
func (h *Handler) Hub() *Hub       { return h.hub }

// Subscribe 处理 GET /api/v1/stream/{channel}。
// 这个 handler 绝不能被 httpx.JSONErrors 包裹（由 httprouter 分流保证）：
// 它依赖真正的 http.Flusher 逐片下发，缓冲拦截器会让 SSE 退化成「等连接关闭才收到」。
func (h *Handler) Subscribe(w http.ResponseWriter, r *http.Request) {
	channel := strings.TrimSpace(r.PathValue("channel"))
	if channel == "" {
		httpx.BadRequest(w, "channel is required")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		httpx.InternalError(w)
		return
	}
	lastID := r.Header.Get("Last-Event-ID")
	if lastID == "" {
		lastID = r.URL.Query().Get("last_event_id") // EventSource 不能自定义头，允许 query 传
	}
	events, unsubscribe, err := h.hub.Subscribe(channel, lastID)
	if err != nil {
		httpx.SystemError(w, http.StatusServiceUnavailable, httpx.CodeSystemNotReady, "stream hub is shutting down")
		return
	}
	defer unsubscribe()

	hdr := w.Header()
	hdr.Set("Content-Type", "text/event-stream")
	hdr.Set("Cache-Control", "no-cache, no-transform")
	hdr.Set("Connection", "keep-alive")
	hdr.Set("X-Accel-Buffering", "no") // 关掉 Nginx 等反代缓冲
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "retry: 3000\n\n") // 断线后 3s 重连
	flusher.Flush()

	ticker := time.NewTicker(h.hub.Heartbeat())
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done(): // 客户端断开
			return
		case <-h.hub.Done(): // 服务关停，主动 drain
			return
		case ev, open := <-events:
			if !open {
				return
			}
			if writeEvent(w, ev) != nil {
				return
			}
			flusher.Flush()
		case <-ticker.C:
			if _, err := io.WriteString(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// Publish 处理 POST /api/v1/stream/{channel}/events：从外部注入一条事件（联调/运维用）。
func (h *Handler) Publish(w http.ResponseWriter, r *http.Request) {
	channel := strings.TrimSpace(r.PathValue("channel"))
	if channel == "" {
		httpx.BadRequest(w, "channel is required")
		return
	}
	var req struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.BadRequest(w, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Type) == "" {
		req.Type = "message"
	}
	if len(req.Data) == 0 {
		req.Data = json.RawMessage(`{}`)
	}
	ev := h.hub.Publish(channel, req.Type, req.Data)
	httpx.OK(w, map[string]any{"id": ev.ID, "channel": channel})
}

func writeEvent(w io.Writer, ev Event) error {
	data, err := json.Marshal(ev.Data)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "id: %s\nevent: %s\ndata: %s\n\n", ev.ID, ev.Type, data)
	return err
}
