// Package platformhttp 的 Huma 集成：在现有 net/http 路由树上并存一个 Huma API，
// 让新业务用 code-first 方式声明接口（类型 → OpenAPI 3.1 + 自动校验），
// 同时复用已有的顶层中间件链（RequestID/CORS/Logger/Limit/Recover/MaxBody）。
package platformhttp

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
)

// HumaConfig 是创建 Huma API 的参数。
type HumaConfig struct {
	Title       string
	Version     string
	Description string
	SpecPath    string // 相对 API 前缀的完整路径（如 /api/v1/openapi.json）
	DocsPath    string
}

// NewHumaAPI 在给定 mux 上创建 Huma API。
// authn 非空时，所有声明了 BearerSecurity 的操作自动走现有鉴权链；
// 权限码从 Operation.Metadata["permission"] 读取。
func NewHumaAPI(mux *http.ServeMux, c HumaConfig, authn HumaAuthenticator) huma.API {
	cfg := huma.DefaultConfig(c.Title, c.Version)
	cfg.OpenAPI.Info.Description = c.Description
	cfg.OpenAPIPath = c.SpecPath
	cfg.DocsPath = c.DocsPath
	cfg.DocsRenderer = "scalar"

	cfg.OpenAPI.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"bearerAuth": {Type: "http", Scheme: "bearer", BearerFormat: "JWT"},
	}
	api := humago.NewWithPrefix(mux, "/api/v1", cfg)
	if authn != nil {
		api.UseMiddleware(autoAuth(authn))
	}
	return api
}

// autoAuth 按操作的 Security 声明自动套鉴权：
// 没有任何非空安全要求（公开）放行；带 bearerAuth 的按 metadata.permission 查权限。
func autoAuth(authn HumaAuthenticator) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		op := ctx.Operation()
		if op == nil || !requiresAuth(op.Security) {
			next(ctx) // 公开接口：security 为空，或只含空 requirement（{}）
			return
		}
		permission, _ := op.Metadata["permission"].(string)
		HumaAuth(authn, permission)(ctx, next)
	}
}

// requiresAuth 判断 security 声明是否真的要求认证。
// 注意：OpenAPI 里 `security: [{}]`（一个空 map）表示「允许匿名」，
// 只有包含非空 scheme 的 requirement 才需要认证——不能只看 len(Security)。
func requiresAuth(security []map[string][]string) bool {
	for _, requirement := range security {
		if len(requirement) > 0 {
			return true
		}
	}
	return false
}

// BearerSecurity 是已登录接口的 OpenAPI security 声明。
func BearerSecurity() []map[string][]string {
	return []map[string][]string{{"bearerAuth": {}}}
}

// PublicSecurity 是公开接口的显式声明：OpenAPI 输出 `security: []`，明确不需要认证。
func PublicSecurity() []map[string][]string {
	return []map[string][]string{}
}
