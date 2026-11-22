// Package platformhttp 提供平台级路由注册器。
//
// 职责边界：
//   - 把「鉴权策略」（公开 / 仅需登录 / 需要权限 / SSE）收口到一处，业务模块只声明级别；
//   - 处理 JSON 与流式（SSE）两种协议的组装差异；
//   - 不含任何业务语义，不认识任何业务 handler。
//
// 这样 httprouter 就退回成「纯路由表」：谁有哪些接口一目了然，不再夹杂鉴权与协议适配。
package platformhttp

import (
	"net/http"
	"reflect"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"stream-hub/internal/httpx"
)

// streamPathPrefix 是 SSE 流路径前缀。GET 请求命中此前缀时走流式 mux，
// 绕过 httpx.JSONErrors（它全量缓冲响应、不实现 Flusher，会破坏 SSE）。
const streamPathPrefix = "/api/v1/stream/"

// Authenticator 是注册器对认证能力的依赖（依赖倒置：只声明要用什么，不依赖具体实现）。
// *auth.Authenticator 天然满足此接口；nil 表示认证未启用，所有关卡直接放行。
type Authenticator interface {
	Middleware(next http.Handler) http.Handler
	RequirePermission(permission string) func(http.Handler) http.Handler
	EnforcePasswordChange(next http.Handler) http.Handler
	MiddlewareQueryToken(next http.Handler) http.Handler
}

// Router 把路由注册与鉴权策略合在一处，最终产出一个 http.Handler。
type Router struct {
	jsonMux   *http.ServeMux
	streamMux *http.ServeMux
	authn     Authenticator
	humaAPI   huma.API
}

func New(authn Authenticator) *Router {
	return &Router{
		jsonMux:   http.NewServeMux(),
		streamMux: http.NewServeMux(),
		authn:     nonNil(authn),
	}
}

// Mux 返回 JSON 路由的底层 mux，供 Huma 等框架在同一路由树上并存。
func (r *Router) Mux() *http.ServeMux { return r.jsonMux }

// SetHumaAPI 记录本路由树上的 Huma API 实例（同一 mux，共享顶层中间件）。
func (r *Router) SetHumaAPI(api huma.API) { r.humaAPI = api }

// HumaAPI 返回已装配的 Huma API；未启用时为 nil。
func (r *Router) HumaAPI() huma.API { return r.humaAPI }

// nonNil 把 typed-nil（如 (*auth.Authenticator)(nil) 装进接口）归一成真正的 nil，
// 避免调用方忘记判断就传入时挂上空指针鉴权关卡。
func nonNil(a Authenticator) Authenticator {
	if a == nil {
		return nil
	}
	v := reflect.ValueOf(a)
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return nil
	}
	return a
}

// Public 注册公开接口（无需登录），如登录入口、登录页要读的展示配置、健康检查。
func (r *Router) Public(pattern string, h http.HandlerFunc) {
	r.jsonMux.Handle(pattern, withPattern(pattern, h))
}

// Handle 注册一个已有的 http.Handler（静态资源、文件服务等）。
func (r *Router) Handle(pattern string, h http.Handler) {
	r.jsonMux.Handle(pattern, withPattern(pattern, h))
}

// NeedAuth 注册只需登录态的接口：验身份即可，不看权限、也不管强制改密。
func (r *Router) NeedAuth(pattern string, h http.HandlerFunc) {
	r.jsonMux.Handle(pattern, withPattern(pattern, r.guard(h, "")))
}

// Authorize 注册需要权限的接口：认证(401) → 强制改密 → 权限判定(403) 三件套。
func (r *Router) Authorize(pattern, permission string, h http.HandlerFunc) {
	r.jsonMux.Handle(pattern, withPattern(pattern, r.guard(h, permission)))
}

// Stream 注册 SSE 流接口：支持 ?token= 查询参数（EventSource 不能带自定义头），
// 只验签不查权限、不套强制改密（长连接返回 JSON 信封无意义）。
func (r *Router) Stream(pattern string, h http.HandlerFunc) {
	var sh http.Handler = h
	if r.authn != nil {
		sh = r.authn.MiddlewareQueryToken(sh)
	}
	r.streamMux.Handle(pattern, withPattern(pattern, sh))
}

// guard 按权限级别包裹 handler。permission 为空表示只挂认证关卡。
func (r *Router) guard(h http.HandlerFunc, permission string) http.Handler {
	var next http.Handler = h
	if r.authn == nil {
		return next
	}
	if permission != "" {
		// 包裹顺序即执行顺序：认证 → 强制改密 → 权限判定 → 业务。
		next = r.authn.RequirePermission(permission)(next)
		next = r.authn.EnforcePasswordChange(next)
	}
	return r.authn.Middleware(next)
}

// Handler 产出最终 handler：GET 命中流前缀走 streamMux，其余走套了 JSONErrors 的 jsonMux。
func (r *Router) Handler() http.Handler {
	jsonHandler := httpx.JSONErrors(r.jsonMux)
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, streamPathPrefix) {
			r.streamMux.ServeHTTP(w, req)
			return
		}
		jsonHandler.ServeHTTP(w, req)
	})
}
