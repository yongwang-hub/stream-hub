// Package middleware 存放中间件。
package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// Logger 中间件：每个请求结束后，打一条结构化访问日志。
//
// 要记 status 和响应字节数，就得「偷看」下游写了什么。这里用的是
// write-through（直传）包装：每个 Write 立刻原样转给真 ResponseWriter，
// 只在旁边顺手记账——不缓冲任何字节，流式响应不受影响。
// （对比：httpx.JSONErrors 那种全量截住改写，会破坏 SSE/流式，
// 详见 docs/响应缓冲与流式问题.md。两种拦截器，用途不同。）
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rec := &respRecorder{ResponseWriter: w, status: 0}
		next.ServeHTTP(rec, r) // 请求照常往下走，rec 只在旁边记账

		// 能走到这里说明没 panic（panic 会被外层 Recover 接住）。
		// 按状态码自动定级：5xx=Error，4xx=Warn，其余=Info。
		level := slog.LevelInfo
		switch {
		case rec.status >= 500:
			level = slog.LevelError
		case rec.status >= 400:
			level = slog.LevelWarn
		}

		slog.Default().LogAttrs(r.Context(), level, "http request",
			slog.String("request_id", GetRequestID(r.Context())), // 串链路的钥匙
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rec.status),
			slog.Int("bytes", rec.bytes),
			slog.Duration("duration", time.Since(start)),
		)
	})
}

// respRecorder 是 write-through 版 ResponseWriter 包装：只记录，不拦截。
type respRecorder struct {
	http.ResponseWriter     // 嵌入：没重写的方法（Header 等）直接透传给真 writer
	status              int // 0 表示还没写过状态码
	bytes               int
}

// WriteHeader 记下状态码，立刻转发。
func (r *respRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// Write 记字节数，数据立刻直传（不缓冲！）。
// 若下游没显式 WriteHeader，按 net/http 规则隐式算 200。
func (r *respRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b) // 直传：真 writer 收多少客户端收多少
	r.bytes += n
	return n, err
}

// Flush 透传给下层——保住 SSE/流式响应能力（实现 http.Flusher 接口）。
// 少了这个方法，包一层就废掉流式：中间件包装 ResponseWriter 的经典暗坑。
func (r *respRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
