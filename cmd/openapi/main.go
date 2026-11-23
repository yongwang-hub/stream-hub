// openapi 导出 Huma 生成的 OpenAPI 3.1 文档到文件，供前端生成 TS 类型与 CI 契约检查。
//
// 用法：go run ./cmd/openapi -out web/openapi.json
// 只装配路由（不连库、不启监听），因此可以在任何环境离线执行。
package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	"stream-hub/internal/camera"
	"stream-hub/internal/db"
	"stream-hub/internal/httprouter"
	"stream-hub/internal/identity"
	"stream-hub/internal/media"
	"stream-hub/internal/metrics"
	"stream-hub/internal/platform/auth"
	"stream-hub/internal/stream"
	"stream-hub/internal/systemconfig"
)

func main() {
	out := flag.String("out", "web/openapi.json", "输出文件路径")
	flag.Parse()

	h := buildHandlerForSpec()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/openapi.json", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		fmt.Fprintf(os.Stderr, "generate spec failed: HTTP %d: %s\n", rec.Code, rec.Body.String())
		os.Exit(1)
	}

	if dir := filepath.Dir(*out); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "mkdir %s: %v\n", dir, err)
			os.Exit(1)
		}
	}
	if err := os.WriteFile(*out, rec.Body.Bytes(), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", *out, err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (%d bytes)\n", *out, rec.Body.Len())
}

// buildHandlerForSpec 用最小依赖装配全部路由：只注册 Huma 操作，不触碰真实数据库。
func buildHandlerForSpec() http.Handler {
	info := systemconfig.NewSystemInfo("dev", "MODEL", "SN", 0)
	return httprouter.New(httprouter.Deps{
		// authn 非 nil 才能让 Huma 注册鉴权声明与认证接口；accounts 为 nil 不影响注册。
		Authn:        auth.New("spec-only-secret", time.Hour, nil),
		Admin:        identity.NewAdminHandler(identity.NewAdminRepo(nil, db.Dialect{})),
		Media:        media.NewHandler(media.NewService(media.NewMemStore(), nil)),
		Camera:       camera.NewHandler(camera.NewService(camera.NewMemStore())),
		Metrics:      metrics.NewHandler(metrics.NewService(metrics.NewCollector(), nil, time.Second, 10)),
		SystemConfig: systemconfig.NewHandler(systemconfig.NewService(systemconfig.NewMemStore(), info), nil),
		Stream:       stream.NewHandler(stream.NewHub(0)),
	})
}
