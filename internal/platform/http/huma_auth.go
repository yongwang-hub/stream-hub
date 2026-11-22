package platformhttp

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
)

// Authenticator 是 Huma 鉴权适配所需的最小能力（与 http.Authenticator 同签名子集）。
type HumaAuthenticator interface {
	Middleware(next http.Handler) http.Handler
	RequirePermission(permission string) func(http.Handler) http.Handler
}

// HumaAuth 返回一个 Huma middleware，用现有 HTTP 鉴权链保护操作。
// permission 为空时只验登录态；非空时还要查权限码。
//
// 实现要点：通过 humago.Unwrap 拿到原生 *http.Request / ResponseWriter，
// 把 Huma 的 next 适配成 http.Handler，再交给已有的 Middleware/RequirePermission。
// 鉴权逻辑因此只有一份，HTTP 路由与 Huma 路由行为完全一致。
func HumaAuth(authn HumaAuthenticator, permission string) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		if authn == nil {
			next(ctx)
			return
		}
		r, w := humago.Unwrap(ctx)
		bridge := http.HandlerFunc(func(_ http.ResponseWriter, nextR *http.Request) {
			// 关键：把鉴权 middleware 注入了 claims 的 context 传回 Huma，
			// 否则 handler 里的 FromContext 拿不到身份，改密等接口会失败。
			next(huma.WithContext(ctx, nextR.Context()))
		})
		h := authn.Middleware(bridge)
		if permission != "" {
			h = authn.RequirePermission(permission)(h)
		}
		h.ServeHTTP(w, r)
	}
}
