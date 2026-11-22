package log

import (
	"context"
	"log/slog"
)

// requestIDHandler 是 slog.Handler 装饰器：从 context 提取关联 ID，
// 自动加到每条日志上。业务代码照常用 slog，不必把 ID 当参数层层透传。
//
// 实现 Handler 接口要求连同 WithAttrs / WithGroup 一起包装，
// 否则 logger.With(...) 产生的子 logger 会丢掉注入能力。
type requestIDHandler struct {
	slog.Handler
	extract RequestIDFunc
}

func (h *requestIDHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := h.extract(ctx); id != "" {
		r.AddAttrs(slog.String("rid", id))
	}
	return h.Handler.Handle(ctx, r)
}

func (h *requestIDHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &requestIDHandler{Handler: h.Handler.WithAttrs(attrs), extract: h.extract}
}

func (h *requestIDHandler) WithGroup(name string) slog.Handler {
	return &requestIDHandler{Handler: h.Handler.WithGroup(name), extract: h.extract}
}
