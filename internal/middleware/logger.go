// Package middleware 存放中间件。
package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	platformhttp "stream-hub/internal/platform/http"
)

// LoggerOptions 控制访问日志的降噪与慢请求告警。
type LoggerOptions struct {
	// QuietPaths 是精确匹配的低价值路径（探针 / favicon），降到 DEBUG，默认不刷屏。
	// 5xx 不受影响：真故障必须看得见。
	QuietPaths []string
	// SlowThreshold >0 时，耗时超阈值的请求自动升一级（INFO→WARN、WARN→ERROR）。
	SlowThreshold time.Duration
}

// Logger 用默认选项；需要降噪/慢请求告警时用 LoggerWith。
func Logger(next http.Handler) http.Handler {
	return LoggerWith(LoggerOptions{})(next)
}

// LoggerWith 每个请求结束后打一条结构化访问日志。
//
// 要记 status 和响应字节数，就得「偷看」下游写了什么。这里用的是
// write-through（直传）包装：每个 Write 立刻原样转给真 ResponseWriter，
// 只在旁边顺手记账——不缓冲任何字节，流式响应不受影响。
// （对比：httpx.JSONErrors 那种全量截住改写，会破坏 SSE/流式，
// 详见 docs/响应缓冲与流式问题.md。两种拦截器，用途不同。）
func LoggerWith(o LoggerOptions) func(http.Handler) http.Handler {
	quiet := make(map[string]struct{}, len(o.QuietPaths))
	for _, p := range o.QuietPaths {
		quiet[p] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// 路由模板容器放 context：内层注册器填值，外层这里读取。
			// 指针共享，auth 中间件用 WithContext 克隆请求也不会丢。
			ctx, route := platformhttp.WithRoutePattern(r.Context())
			r = r.WithContext(ctx)

			rec := &respRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r) // 请求照常往下走，rec 只在旁边记账

			elapsed := time.Since(start)
			path := r.URL.Path
			if route.Value != "" {
				path = route.Value // 用模板而非原始路径，避免 /users/1、/users/2 刷屏
				if i := strings.IndexByte(path, ' '); i >= 0 {
					path = path[i+1:] // 去掉 "GET " 方法前缀，method 字段已有
				}
			}

			slog.Default().LogAttrs(r.Context(),
				accessLevel(rec.status, elapsed, path, quiet, o.SlowThreshold),
				"http",
				slog.Int("status", rec.status),
				slog.String("method", r.Method),
				slog.String("route", path),
				slog.Int("bytes", rec.bytes),
				slog.Duration("duration", elapsed),
				// request_id 由 platform/log 的 handler 从 context 自动注入，不在这里重复打。
			)
		})
	}
}

// accessLevel 决定访问日志级别：先看状态码，再应用降噪与慢请求规则。
func accessLevel(status int, elapsed time.Duration, path string, quiet map[string]struct{}, slow time.Duration) slog.Level {
	level := slog.LevelInfo
	switch {
	case status >= 500:
		level = slog.LevelError
	case status >= 400:
		level = slog.LevelWarn
	}

	if _, ok := quiet[path]; ok && status < 500 {
		level = slog.LevelDebug // 探针/favicon 的 4xx 是常态，不是问题
	}
	if slow > 0 && elapsed >= slow && level < slog.LevelWarn {
		level = slog.LevelWarn // 慢请求升级，但仍保留 5xx 的 Error 级别
	}
	return level
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
