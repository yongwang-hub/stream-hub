package platformhttp

import (
	"context"
	"net/http"
)

// RoutePattern 是路由模板的容器，放在请求 context 里由注册器填入。
// 用指针而非字符串：外层的日志中间件先放入空容器，内层路由匹配后填值，
// 即使中间有中间件调用 r.WithContext() 克隆请求，容器指针仍然共享。
type RoutePattern struct {
	Value string
}

type routePatternKey struct{}

// WithRoutePattern 在 context 里放入一个空容器，返回容器供外层读取结果。
func WithRoutePattern(ctx context.Context) (context.Context, *RoutePattern) {
	p := &RoutePattern{}
	return context.WithValue(ctx, routePatternKey{}, p), p
}

// PatternFromContext 读取路由模板；未经过注册器时返回空串。
func PatternFromContext(ctx context.Context) string {
	p, _ := ctx.Value(routePatternKey{}).(*RoutePattern)
	if p == nil {
		return ""
	}
	return p.Value
}

// withPattern 把注册时的 pattern 写进容器，再调用真正的 handler。
func withPattern(pattern string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p, ok := r.Context().Value(routePatternKey{}).(*RoutePattern); ok && p != nil {
			p.Value = pattern
		}
		h.ServeHTTP(w, r)
	})
}
