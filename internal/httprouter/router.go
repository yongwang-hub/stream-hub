// Package httprouter 负责装配路由：把「URL 路径 + HTTP 方法」映射到处理函数。
//
// 阶段 1 的硬约束：SSE 路由必须绕过 httpx.JSONErrors（它全量缓冲响应、不实现 Flusher），
// 因此这里用「两个 mux + 路径分流」：普通 JSON 路由走包裹后的 mux，
// GET /api/v1/stream/* 直接走流式 mux，保证 http.Flusher 是真正的下游 writer。
package httprouter

import (
	"context"
	"net/http"
	"strings"
	"time"

	"stream-hub/internal/auth"
	"stream-hub/internal/httpx"
	"stream-hub/internal/media"
	"stream-hub/internal/metrics"
	"stream-hub/internal/stream"
	"stream-hub/internal/systemconfig"
	"stream-hub/internal/user"
)

// Deps 是组合根注入的全部依赖；nil 表示对应能力未启用。
type Deps struct {
	User         *user.Handler
	Media        *media.Handler
	Stream       *stream.Handler
	Authn        *auth.Authenticator
	Admin        *auth.AdminHandler    // 真实账号/角色管理接口；nil 表示未启用认证时不暴露
	Metrics      *metrics.Handler      // 系统指标快照；实时增量走 system-metrics SSE 频道
	SystemConfig *systemconfig.Handler // 运行时系统配置（读登录即可，写需 config:manage 权限）
	Ready        func(context.Context) error
	DebugRoutes  bool
	Static       http.Handler // 运行时从磁盘 dist/ 读取的静态文件服务；nil 表示不托管页面
}

const streamPathPrefix = "/api/v1/stream/"

// New 创建装配好路由的 handler。
func New(d Deps) http.Handler {
	mux := http.NewServeMux()

	// 登录 / 改密：登录入口公开，改密只需登录态。
	if d.Authn != nil {
		mux.Handle("POST /login", http.HandlerFunc(d.Authn.Login))
		mux.Handle("PUT /me/password", needAuth(d.Authn, d.Authn.ChangePassword))
	}

	// 用户 demo CRUD（阶段 1 保留，后续由业务模块取代）。
	if d.User != nil {
		mux.Handle("POST /users", authorize(d.Authn, auth.PermUserCreate, d.User.Create))
		mux.Handle("GET /users", authorize(d.Authn, auth.PermUserList, d.User.List))
		mux.Handle("GET /users/{id}", authorize(d.Authn, auth.PermUserRead, d.User.Get))
		mux.Handle("PUT /users/{id}", authorize(d.Authn, auth.PermUserUpdate, d.User.Update))
		mux.Handle("DELETE /users/{id}", authorize(d.Authn, auth.PermUserDelete, d.User.Delete))
	}

	// 真实账号体系管理接口：操作 sys_users / sys_roles，走 RBAC 权限码。
	// 与上面的 demo /users 完全分离，前端「用户管理 / 角色管理」两页对接这里。
	if d.Admin != nil {
		mux.Handle("GET /api/v1/users", authorize(d.Authn, auth.PermUserList, d.Admin.ListUsers))
		mux.Handle("POST /api/v1/users", authorize(d.Authn, auth.PermUserCreate, d.Admin.CreateUser))
		mux.Handle("PUT /api/v1/users/{id}", authorize(d.Authn, auth.PermUserUpdate, d.Admin.UpdateUser))
		mux.Handle("DELETE /api/v1/users/{id}", authorize(d.Authn, auth.PermUserDelete, d.Admin.DeleteUser))

		mux.Handle("GET /api/v1/roles", authorize(d.Authn, auth.PermRoleManage, d.Admin.ListRoles))
		mux.Handle("POST /api/v1/roles", authorize(d.Authn, auth.PermRoleManage, d.Admin.CreateRole))
		mux.Handle("PUT /api/v1/roles/{id}", authorize(d.Authn, auth.PermRoleManage, d.Admin.UpdateRole))
		mux.Handle("DELETE /api/v1/roles/{id}", authorize(d.Authn, auth.PermRoleManage, d.Admin.DeleteRole))
	}

	// 系统指标快照：只做认证（登录即可查看监控），实时增量复用 system-metrics SSE 频道。
	if d.Metrics != nil {
		mux.Handle("GET /api/v1/metrics", needAuth(d.Authn, d.Metrics.Snapshot))
	}

	// 系统配置：读公开（登录页需要产品名称/图标/背景渲染，这些是展示型数据不含敏感信息）；
	// 写与图片上传需要 config:manage 权限。
	if d.SystemConfig != nil {
		mux.Handle("GET /api/v1/system-config", http.HandlerFunc(d.SystemConfig.Get))
		mux.Handle("PUT /api/v1/system-config", authorize(d.Authn, auth.PermConfigManage, d.SystemConfig.Update))
		mux.Handle("POST /api/v1/system-config/upload", authorize(d.Authn, auth.PermConfigManage, d.SystemConfig.Upload))
	}

	// 健康检查：liveness 不做任何依赖检查，readiness 才碰 DB。
	mux.HandleFunc("GET /ping", ping)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		httpx.OK(w, "ok")
	})
	mux.HandleFunc("GET /readyz", readyz(d.Ready))

	// 频道 CRUD：读只需登录态，写需要 channel:manage。
	if d.Media != nil {
		mux.Handle("GET /api/v1/channels", needAuth(d.Authn, d.Media.List))
		mux.Handle("GET /api/v1/channels/{id}", needAuth(d.Authn, d.Media.Get))
		mux.Handle("POST /api/v1/channels", authorize(d.Authn, auth.PermChannelManage, d.Media.Create))
		mux.Handle("PUT /api/v1/channels/{id}", authorize(d.Authn, auth.PermChannelManage, d.Media.Update))
		mux.Handle("DELETE /api/v1/channels/{id}", authorize(d.Authn, auth.PermChannelManage, d.Media.Delete))
	}
	// 外部事件注入（联调/运维）：虽与 SSE 同前缀，但它是 JSON 响应，仍走 JSONErrors。
	if d.Stream != nil {
		mux.Handle("POST /api/v1/stream/{channel}/events", authorize(d.Authn, auth.PermStreamPublish, d.Stream.Publish))
	}

	// 演示路由：仅本地开发注册，生产由 server.debug_routes=false 关掉。
	if d.DebugRoutes {
		mux.HandleFunc("GET /panic", func(w http.ResponseWriter, r *http.Request) {
			panic("boom")
		})
		mux.HandleFunc("GET /slow", func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(5 * time.Second)
			httpx.OK(w, "done after 5s")
		})
	}

	// 前端静态产物（独立 dist 目录，运行时从磁盘读取）：只暴露根路径与带内容 hash 的 /assets/。
	// 其余路径仍由 ServeMux 404 → JSONErrors 转 JSON 信封，API 404 不会被吞成 HTML。
	if d.Static != nil {
		mux.Handle("GET /{$}", d.Static)
		mux.Handle("GET /assets/", d.Static)
	}

	// 上传的图片（系统配置里的背景/图标）：落盘在 data/uploads/，按相对 URL 提供。
	// 图片本身不含敏感信息，走公开静态服务；文件名是随机串，不可枚举。
	mux.Handle("GET /uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("data/uploads"))))

	// 流式 mux：绝不套 JSONErrors。
	streamMux := http.NewServeMux()
	if d.Stream != nil {
		var sh http.Handler = http.HandlerFunc(d.Stream.Subscribe)
		if d.Authn != nil {
			sh = d.Authn.MiddlewareQueryToken(sh) // 支持 ?token=，EventSource 不能带自定义头
		}
		streamMux.Handle("GET /api/v1/stream/{channel}", sh)
	}

	jsonHandler := httpx.JSONErrors(mux)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, streamPathPrefix) {
			streamMux.ServeHTTP(w, r)
			return
		}
		jsonHandler.ServeHTTP(w, r)
	})
}

// needAuth 只挂认证关卡（Middleware）：能证明身份即可，不看权限、也不管强制改密。
func needAuth(authn *auth.Authenticator, h http.HandlerFunc) http.Handler {
	if authn == nil {
		return h
	}
	return authn.Middleware(h)
}

// authorize 挂完整三件套：认证(401) → 强制改密关卡 → 权限判定(403)。
func authorize(authn *auth.Authenticator, permission string, h http.HandlerFunc) http.Handler {
	if authn == nil {
		return h
	}
	return authn.Middleware(
		authn.EnforcePasswordChange(
			authn.RequirePermission(permission)(h),
		),
	)
}

func ping(w http.ResponseWriter, r *http.Request) {
	httpx.OK(w, "pong")
}

func readyz(ready func(context.Context) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if ready != nil {
			if err := ready(r.Context()); err != nil {
				httpx.SystemError(w, http.StatusServiceUnavailable, httpx.CodeSystemNotReady, "not ready")
				return
			}
		}
		httpx.OK(w, "ready")
	}
}
