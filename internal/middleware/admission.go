// Limit 并发准入控制（admission control）中间件：给「同时在处理的请求数」设一道硬上限。
//
// 为什么性能优先的系统尤其需要它：
// 吞吐不是越高越好——超过某个并发后，连接池耗尽、GC 抖动、上下文切换会让 P99 雪崩，
// 最后连正常请求一起拖死（过载雪崩）。与其「扛到崩」，不如在门口就干脆拒绝多余的请求，
// 让服务稳在最佳吞吐点：进来的都能快速返回，超额的立刻 429 走人。这叫「优雅降级」。
//
// 实现只用一个原子计数器（无锁、零分配），在热路径上的开销可忽略——符合本项目轻依赖取向。
package middleware

import (
	"net/http"
	"sync/atomic"

	"stream-hub/internal/httpx"
)

// Limit 限制最多 max 个请求同时在处理；第 max+1 个进来立即 429（不排队、不占资源）。
// max <= 0 表示不限——此时返回一个零开销的透传，不给热路径加任何原子操作。
// bypass 可选：返回 true 的请求（如 SSE 长连接）完全不占 slot——否则一条长连接会一直占着额度。
func Limit(max int, bypass ...func(*http.Request) bool) Middleware {
	if max <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}

	var shouldBypass func(*http.Request) bool
	if len(bypass) > 0 {
		shouldBypass = bypass[0]
	}
	var inFlight atomic.Int64
	limit := int64(max)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if shouldBypass != nil && shouldBypass(r) {
				next.ServeHTTP(w, r)
				return
			}
			// 原子地「先占位再看超没超」：Add 返回自增后的值。
			// 超额则立刻撤销占位并拒绝——绝不把 slot 留给注定失败的请求。
			if inFlight.Add(1) > limit {
				inFlight.Add(-1)
				httpx.TooBusy(w)
				return
			}
			// 占到 slot：请求处理完（含 panic 被外层 Recover 接住）都要归还。
			defer inFlight.Add(-1)
			next.ServeHTTP(w, r)
		})
	}
}
