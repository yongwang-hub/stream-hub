// Recover 中间件：在自己这一层就接住下游的 panic，
// 用我们自己的方式记录，并给客户端回一个规范的 500。
package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"stream-hub/internal/httpx"
)

// 形状和 Logger 完全一样：吃一个 next，吐一个包了皮的新 handler。
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// defer 保证：不管 next 正常返回还是 panic，这段都会在「本层结束前」执行。
		// recover() 只有在 defer 中才有效；接住了返回 panic 的值，没接住返回 nil。
		defer func() {
			if rec := recover(); rec != nil {
				// 结构化记录：带请求上下文和完整堆栈（堆栈才是排查 panic 的命根子）。
				slog.Default().Error("panic recovered",
					"request_id", GetRequestID(r.Context()),
					"method", r.Method,
					"path", r.URL.Path,
					"panic", rec,
					"stack", string(debug.Stack()),
				)
				// 给客户端一个规范的 500，走统一信封（不透露内部 panic 细节）。
				httpx.InternalError(w)
			}
		}()

		next.ServeHTTP(w, r) // 照常把请求交给里面的路由
	})
}
