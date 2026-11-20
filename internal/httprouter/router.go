// Package httprouter 负责装配路由：把「URL 路径 + HTTP 方法」映射到处理函数。
//
// 本包现在是纯路由表：只声明「有哪些接口、各自什么鉴权级别」，
// 鉴策略与 JSON/SSE 协议分流由 internal/platform/http.Router 统一处理。
package httprouter

import (
	"context"
	"net/http"
	"time"

	"stream-hub/internal/auth"
	"stream-hub/internal/httpx"
	"stream-hub/internal/media"
	"stream-hub/internal/metrics"
	platformhttp "stream-hub/internal/platform/http"
	"stream-hub/internal/stream"
	"stream-hub/internal/systemconfig"
)

// Deps 是组合根注入的全部依赖；nil 表示对应能力未启用。
type Deps struct {
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

// New 创建装配好路由的 handler。
func New(d Deps) http.Handler {
	// 注意 typed-nil：*Authenticator(nil) 直接传给接口参数后接口非 nil，
	// 会让注册器误以为认证已启用而挂上鉴权关卡。只在真正非 nil 时才赋值接口。
	var authn platformhttp.Authenticator
	if d.Authn != nil {
		authn = d.Authn
	}
	r := platformhttp.New(authn)

	// 登录 / 改密：登录入口公开，改密只需登录态。
	if d.Authn != nil {
		r.Public("POST /login", d.Authn.Login)
		r.NeedAuth("PUT /me/password", d.Authn.ChangePassword)
	}

	// 账号体系管理接口：操作 sys_users / sys_roles，走 RBAC 权限码。
	if d.Admin != nil {
		r.Authorize("GET /api/v1/users", auth.PermUserList, d.Admin.ListUsers)
		r.Authorize("POST /api/v1/users", auth.PermUserCreate, d.Admin.CreateUser)
		r.Authorize("PUT /api/v1/users/{id}", auth.PermUserUpdate, d.Admin.UpdateUser)
		r.Authorize("DELETE /api/v1/users/{id}", auth.PermUserDelete, d.Admin.DeleteUser)

		r.Authorize("GET /api/v1/roles", auth.PermRoleManage, d.Admin.ListRoles)
		r.Authorize("POST /api/v1/roles", auth.PermRoleManage, d.Admin.CreateRole)
		r.Authorize("PUT /api/v1/roles/{id}", auth.PermRoleManage, d.Admin.UpdateRole)
		r.Authorize("DELETE /api/v1/roles/{id}", auth.PermRoleManage, d.Admin.DeleteRole)
	}

	// 系统指标快照：只做认证（登录即可查看监控），实时增量复用 system-metrics SSE 频道。
	if d.Metrics != nil {
		r.NeedAuth("GET /api/v1/metrics", d.Metrics.Snapshot)
	}

	// 系统配置：读公开（登录页需要产品名称/图标/背景渲染，这些是展示型数据不含敏感信息）；
	// 写与图片上传需要 config:manage 权限。
	if d.SystemConfig != nil {
		r.Public("GET /api/v1/system-config", d.SystemConfig.Get)
		r.Authorize("PUT /api/v1/system-config", auth.PermConfigManage, d.SystemConfig.Update)
		r.Authorize("POST /api/v1/system-config/upload", auth.PermConfigManage, d.SystemConfig.Upload)
	}

	// 健康检查：liveness 不做任何依赖检查，readiness 才碰 DB。
	r.Public("GET /ping", ping)
	r.Public("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		httpx.OK(w, "ok")
	})
	r.Public("GET /readyz", readyz(d.Ready))

	// 频道 CRUD：读只需登录态，写需要 channel:manage。
	if d.Media != nil {
		r.NeedAuth("GET /api/v1/channels", d.Media.List)
		r.NeedAuth("GET /api/v1/channels/{id}", d.Media.Get)
		r.Authorize("POST /api/v1/channels", auth.PermChannelManage, d.Media.Create)
		r.Authorize("PUT /api/v1/channels/{id}", auth.PermChannelManage, d.Media.Update)
		r.Authorize("DELETE /api/v1/channels/{id}", auth.PermChannelManage, d.Media.Delete)
	}
	// 外部事件注入（联调/运维）：虽与 SSE 同前缀，但它是 JSON 响应，仍走 JSONErrors。
	if d.Stream != nil {
		r.Authorize("POST /api/v1/stream/{channel}/events", auth.PermStreamPublish, d.Stream.Publish)
	}

	// SSE 流：走流式注册器，支持 ?token=，绕过 JSONErrors。
	if d.Stream != nil {
		r.Stream("GET /api/v1/stream/{channel}", d.Stream.Subscribe)
	}

	// 演示路由：仅本地开发注册，生产由 server.debug_routes=false 关掉。
	if d.DebugRoutes {
		r.Public("GET /panic", func(w http.ResponseWriter, _ *http.Request) {
			panic("boom")
		})
		r.Public("GET /slow", func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(5 * time.Second)
			httpx.OK(w, "done after 5s")
		})
	}

	// 前端静态产物（独立 dist 目录，运行时从磁盘读取）：只暴露根路径与带内容 hash 的 /assets/。
	if d.Static != nil {
		r.Handle("GET /{$}", d.Static)
		r.Handle("GET /assets/", d.Static)
	}

	// 上传的图片（系统配置里的背景/图标）：落盘在 data/uploads/，按相对 URL 提供。
	// 图片本身不含敏感信息，走公开静态服务；文件名是随机串，不可枚举。
	r.Handle("GET /uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("data/uploads"))))

	return r.Handler()
}

func ping(w http.ResponseWriter, _ *http.Request) {
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
