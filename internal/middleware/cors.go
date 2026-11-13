// Package middleware：CORS 跨域中间件。
//
// 白名单精确匹配 Origin（不做反射式通配），预检 OPTIONS 直接 204 返回，
// 不进业务 handler；带 credentials 时禁止 "*"（config.Validate 也会在启动期拒绝）。
package middleware

import (
	"net/http"

	"stream-hub/internal/httpx"
)

// CORS 按白名单给响应加跨域头。origins 为空 = 不处理任何跨域请求。
func CORS(origins []string, allowCredentials bool) Middleware {
	allowed := make(map[string]struct{}, len(origins))
	wildcard := false
	for _, o := range origins {
		if o == "*" {
			wildcard = true
			continue
		}
		allowed[o] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" { // 非浏览器请求（curl / 服务端）无需处理
				next.ServeHTTP(w, r)
				return
			}
			match := ""
			if _, ok := allowed[origin]; ok {
				match = origin
			} else if wildcard && !allowCredentials {
				match = "*"
			}
			if match == "" {
				if r.Method == http.MethodOptions {
					httpx.SystemError(w, http.StatusForbidden, httpx.CodeSystemForbidden, "origin not allowed")
					return
				}
				next.ServeHTTP(w, r) // 不写 CORS 头，由浏览器自行拦截
				return
			}

			h := w.Header()
			h.Set("Access-Control-Allow-Origin", match)
			h.Add("Vary", "Origin")
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Last-Event-ID")
			h.Set("Access-Control-Expose-Headers", "X-Request-Id")
			if allowCredentials {
				h.Set("Access-Control-Allow-Credentials", "true")
			}
			if r.Method == http.MethodOptions {
				h.Set("Access-Control-Max-Age", "600")
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
