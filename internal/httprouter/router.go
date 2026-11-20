// Package httprouter 负责装配路由：把各业务模块的自注册路由、平台探针、静态资源聚合成一个 handler。
//
// 本包是纯组装器：业务接口由各模块的 Register 方法声明（鉴权级别也在那里），
// 本包只负责跨模块的共性问题——健康检查、调试路由、静态文件、上传目录。
package httprouter

import (
	"context"
	"net/http"
	"time"

	"stream-hub/internal/httpx"
	"stream-hub/internal/identity"
	"stream-hub/internal/media"
	"stream-hub/internal/metrics"
	"stream-hub/internal/platform/auth"
	platformhttp "stream-hub/internal/platform/http"
	"stream-hub/internal/stream"
	"stream-hub/internal/systemconfig"
)

// Deps 是组合根注入的全部依赖；nil 表示对应能力未启用。
type Deps struct {
	Authn        *auth.Authenticator
	Admin        *identity.AdminHandler
	Media        *media.Handler
	Stream       *stream.Handler
	Metrics      *metrics.Handler
	SystemConfig *systemconfig.Handler
	Ready        func(context.Context) error
	DebugRoutes  bool
	Static       http.Handler
}

// New 创建装配好路由的 handler。
func New(d Deps) http.Handler {
	// typed-nil 防御：*Authenticator(nil) 传进接口会非 nil，需显式转。
	var authn platformhttp.Authenticator
	if d.Authn != nil {
		authn = d.Authn
	}
	r := platformhttp.New(authn)

	// 各业务模块自注册自己的路由与鉴权级别。
	if d.Authn != nil {
		d.Authn.Register(r)
	}
	if d.Admin != nil {
		d.Admin.Register(r)
	}
	if d.Metrics != nil {
		d.Metrics.Register(r)
	}
	if d.SystemConfig != nil {
		d.SystemConfig.Register(r)
	}
	if d.Media != nil {
		d.Media.Register(r)
	}
	if d.Stream != nil {
		d.Stream.Register(r)
	}

	// 平台级探针：liveness 不做依赖检查，readiness 才碰 DB。
	r.Public("GET /ping", ping)
	r.Public("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		httpx.OK(w, "ok")
	})
	r.Public("GET /readyz", readyz(d.Ready))

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

	// 上传的图片：落盘在 data/uploads/，按相对 URL 提供。文件名随机，不可枚举。
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
