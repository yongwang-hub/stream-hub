package config

import (
	"strings"
	"testing"
)

func TestEnvOverrides(t *testing.T) {
	t.Setenv("STREAM_HUB_ADDR", "127.0.0.1:9999")
	t.Setenv("STREAM_HUB_CORS_ORIGINS", "https://a.example, https://b.example")
	t.Setenv("STREAM_HUB_DEBUG_ROUTES", "true")
	t.Setenv("STREAM_HUB_AUTH_ENABLED", "false")

	cfg, err := Load("") // 跳过文件层，只看默认值 + env
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.Addr != "127.0.0.1:9999" {
		t.Errorf("addr = %q", cfg.Server.Addr)
	}
	if !cfg.Server.DebugRoutes {
		t.Error("debug_routes 应被 env 打开")
	}
	if len(cfg.CORS.AllowOrigins) != 2 || cfg.CORS.AllowOrigins[1] != "https://b.example" {
		t.Errorf("cors origins = %v", cfg.CORS.AllowOrigins)
	}
	if cfg.Auth.Secret != "" {
		t.Error("AUTH_ENABLED=false 应关闭认证")
	}
}

func TestValidateRejectsWildcardWithCredentials(t *testing.T) {
	cfg := Default()
	cfg.CORS.AllowOrigins = []string{"*"}
	cfg.CORS.AllowCredentials = true
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "cors") {
		t.Errorf("应拒绝 '*' + credentials，实际 %v", err)
	}
}

func TestValidateProductionRefusesDevSecret(t *testing.T) {
	t.Setenv("STREAM_HUB_ENV", "production")
	cfg := Default()
	cfg.Auth.Secret = "dev-secret-change-me"
	if err := cfg.Validate(); err == nil {
		t.Fatal("生产环境不应允许 dev- 前缀密钥")
	}
	cfg.Auth.Secret = "a-real-secret-from-env"
	cfg.Server.DebugRoutes = true
	if err := cfg.Validate(); err == nil {
		t.Error("生产环境不应允许调试路由")
	}
}
