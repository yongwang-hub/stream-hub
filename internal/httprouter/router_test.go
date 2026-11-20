package httprouter

import (
	"bufio"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"stream-hub/internal/stream"
	"stream-hub/internal/systemconfig"
)

func TestHealthEndpoints(t *testing.T) {
	h := New(Deps{})
	for _, path := range []string{"/ping", "/healthz", "/readyz"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"code":0`) {
			t.Errorf("%s: status=%d body=%s", path, w.Code, w.Body.String())
		}
	}
}

func TestReadyzReturns503WhenDependencyDown(t *testing.T) {
	h := New(Deps{Ready: func(context.Context) error { return errors.New("db down") }})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), `"code":90009`) {
		t.Errorf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestDebugRoutesDisabledByDefault(t *testing.T) {
	h := New(Deps{})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("调试路由默认不该注册，实际 %d", w.Code)
	}
}

// TestSSEStreamsWithoutBuffering 是阶段 1 最关键的回归测试：
// 若 SSE 被 httpx.JSONErrors 全量缓冲，客户端连响应头都收不到，这里会超时失败。
func TestSSEStreamsWithoutBuffering(t *testing.T) {
	hub := stream.NewHub(200 * time.Millisecond)
	h := New(Deps{Stream: stream.NewHandler(hub)})
	srv := httptest.NewServer(h)
	defer srv.Close()

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(srv.URL + "/api/v1/stream/room")
	if err != nil {
		t.Fatalf("建立 SSE 连接失败（可能被缓冲）: %v", err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("Content-Type = %q", ct)
	}

	lines := make(chan string, 64)
	go func() {
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		close(lines)
	}()

	select {
	case line := <-lines:
		if line != "retry: 3000" {
			t.Errorf("首个 SSE 包应为 retry: 3000，实际 %q", line)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("首包被缓冲，SSE 流式通路失效")
	}

	hub.Publish("room", "channel.created", map[string]any{"id": 1})
	deadline := time.After(2 * time.Second)
	for {
		select {
		case line, ok := <-lines:
			if !ok {
				t.Fatal("流意外关闭")
			}
			if line == "event: channel.created" {
				hub.Close()
				return
			}
		case <-deadline:
			t.Fatal("未收到发布事件")
		}
	}
}

func TestSystemConfigRoutesMounted(t *testing.T) {
	info := systemconfig.NewSystemInfo("V1.0.0", "MODEL-X", "SN-1", 10)
	svc := systemconfig.NewService(systemconfig.NewMemStore(), info)
	h := New(Deps{SystemConfig: systemconfig.NewHandler(svc, nil)}) // auth=nil → 直通，便于验证挂载

	// GET 应返回默认配置（含只读元数据）。
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/system-config", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"product_name"`) {
		t.Fatalf("GET system-config: status=%d body=%s", w.Code, w.Body.String())
	}

	// PUT 应更新。
	w = httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/api/v1/system-config", strings.NewReader(`{"product_name":"路由测试平台"}`))
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "路由测试平台") {
		t.Fatalf("PUT system-config: status=%d body=%s", w.Code, w.Body.String())
	}

	// upload 路由存在（空 body 会回业务码而非 404）。
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/system-config/upload", nil))
	if w.Code == http.StatusNotFound {
		t.Fatal("upload route not mounted")
	}

	// /uploads/ 静态路由存在（空目录访问不应 404 路由层面）。
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/uploads/nope.webp", nil))
	if w.Code == http.StatusNotFound && strings.Contains(w.Body.String(), "not found") && !strings.Contains(w.Body.String(), `"code"`) {
		t.Fatal("/uploads static route not mounted")
	}
}
