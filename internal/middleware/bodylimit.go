// MaxBody 请求体大小限制中间件：挡住「POST 一个巨型 body 把内存打爆」这类攻击。
//
// 为什么做成中间件而不是塞进每个 handler：
// 这是一道「进业务逻辑之前」的统一关卡，挂在链上就对所有路由（含将来新增的）生效，
// 不用每个读 body 的地方各写一遍——和 RequestID / Recover 是同一个思路。
package middleware

import (
	"net/http"
	"strconv"

	"stream-hub/internal/httpx"
)

// MaxBody 给每个请求的 body 套上 maxBytes 上限，两道防线各司其职：
//
//  1. 先看 Content-Length 头：客户端「声明」得太大，直接 413 拒绝，连 body 都不碰——最省。
//  2. 再用 http.MaxBytesReader 包一层 r.Body 兜底：应对不声明 Content-Length(分块传输)
//     的偷跑者。它不是提前拦截，而是「真读超了」才让读取报错；届时下游 json.Decode
//     会失败，按框架层 400 出去。
//
// 也就是说：老实的大 body 得到准确的 413，分块伪装的超限表现为 400——
// 两者都拦得住，只是状态码来源不同。要 100% 统一成 413，可在 decode 处识别
// *http.MaxBytesError 再翻成 PayloadTooLarge（本项目 demo 保持简单）。
func MaxBody(maxBytes int64) Middleware {
	limit := strconv.FormatInt(maxBytes, 10)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 防线一：Content-Length 明确超限，立刻 413，不必再读。
			// 注意 r.ContentLength 可能是 -1(未知/分块)或 0(无 body)，都不会误入此分支。
			if r.ContentLength > maxBytes {
				httpx.PayloadTooLarge(w, "request body exceeds "+limit+" bytes")
				return
			}
			// 防线二：包一层硬上限。必须把当前 w 传进去——
			// 超限时 MaxBytesReader 要借 w 通知底层 server 提前结束这次请求。
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}
