// Package httprouter 负责装配路由：把各业务模块的自注册路由、平台探针、静态资源聚合成一个 handler。
//
// 本包是纯组装器：业务接口由各模块的 Register 方法声明（鉴权级别也在那里），
// 本包只负责跨模块的共性问题——健康检查、调试路由、静态文件、上传目录。
package httprouter

import (
	"context"
	"net/http"
	"time"

	"stream-hub/internal/camera"
	"stream-hub/internal/httpx"
	"stream-hub/internal/identity"
	"stream-hub/internal/media"
	"stream-hub/internal/metrics"
	"stream-hub/internal/platform/auth"
	platformhttp "stream-hub/internal/platform/http"
	"stream-hub/internal/platform/storage"
	"stream-hub/internal/stream"
	"stream-hub/internal/systemconfig"
)

// Deps 是组合根注入的全部依赖；nil 表示对应能力未启用。
type Deps struct {
	Authn        *auth.Authenticator
	Admin        *identity.AdminHandler
	Media        *media.Handler
	Camera       *camera.Handler
	Stream       *stream.Handler
	Metrics      *metrics.Handler
	SystemConfig *systemconfig.Handler
	Ready        func(context.Context) error
	DebugRoutes  bool
	Static       http.Handler
	UploadDir    string // 上传文件落盘目录；空则用 storage.DefaultDir
}

// New 创建装配好路由的 handler。
func New(d Deps) http.Handler {
	// typed-nil 防御：*Authenticator(nil) 传进接口会非 nil，需显式转。
	var authn platformhttp.Authenticator
	if d.Authn != nil {
		authn = d.Authn
	}
	r := platformhttp.New(authn)

	// Huma API 与现有注册器共用同一个 mux 和顶层中间件链。
	// 试点阶段只把 systemconfig 迁到 Huma（code-first OpenAPI + 自动校验），
	// 其余模块仍走各自 Register。
	var humaAuthn platformhttp.HumaAuthenticator
	if d.Authn != nil {
		humaAuthn = d.Authn
	}
	humaAPI := platformhttp.NewHumaAPI(r.Mux(), platformhttp.HumaConfig{
		Title:       "stream-hub API",
		Version:     "1.0.0",
		Description: "流媒体数据服务接口；由 Go 类型自动生成 OpenAPI 3.1。",
		SpecPath:    "/openapi",
		DocsPath:    "/docs",
	}, humaAuthn)
	r.SetHumaAPI(humaAPI)
	if d.SystemConfig != nil {
		d.SystemConfig.RegisterHuma(humaAPI)
	}
	if d.Authn != nil {
		d.Authn.RegisterHuma(humaAPI)
	}
	if d.Admin != nil {
		d.Admin.RegisterHuma(humaAPI)
	}
	if d.Media != nil {
		d.Media.RegisterHuma(humaAPI)
	}
	if d.Camera != nil {
		d.Camera.RegisterHuma(humaAPI)
	}
	if d.Metrics != nil {
		d.Metrics.RegisterHuma(humaAPI)
	}
	if d.Stream != nil {
		d.Stream.RegisterHuma(humaAPI)
	}

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

	// 上传的图片：目录与 URL 前缀由平台层 storage 统一提供，避免 handler 与 router 各写一份。
	uploadDir := d.UploadDir
	if uploadDir == "" {
		uploadDir = storage.DefaultDir
	}
	r.Handle("GET "+storage.DefaultURLPath, http.StripPrefix(storage.DefaultURLPath, http.FileServer(http.Dir(uploadDir))))

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
