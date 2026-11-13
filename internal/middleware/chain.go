// Chain：把多个中间件按「正向顺序」组合成一个，避免手写层层嵌套的括号。
package middleware

import "net/http"

// Middleware 给「中间件的函数签名」起个名字，纯为可读性。
// 它和 Logger、Recover 的形状完全一致：吃一个 handler，吐一个包好的新 handler。
type Middleware func(http.Handler) http.Handler

// Chain 接收任意个中间件，返回「一个组合好的中间件」。
// 约定：传入顺序 = 请求进入时的执行顺序（最先传的在最外层）。
//
// 用法：handler := middleware.Chain(Recover, Logger)(路由handler)
func Chain(ms ...Middleware) Middleware {
	return func(final http.Handler) http.Handler {
		// 从后往前套：先把最后一个中间件裹住 final，再往前……
		// 这样最先传入的会落在最外层，符合我们的顺序约定。
		for i := len(ms) - 1; i >= 0; i-- {
			final = ms[i](final)
		}
		return final
	}
}
