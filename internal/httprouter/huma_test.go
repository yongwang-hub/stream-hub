package httprouter

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"stream-hub/internal/db"
	"stream-hub/internal/identity"
	"stream-hub/internal/media"
	"stream-hub/internal/metrics"
	"stream-hub/internal/platform/auth"
	"stream-hub/internal/stream"
	"stream-hub/internal/systemconfig"
)

func newTestDB(t *testing.T) (*sql.DB, db.Dialect) {
	t.Helper()
	d, ok := db.For("sqlite")
	if !ok {
		t.Fatal("sqlite dialect unavailable")
	}
	dbs, err := d.Open("file:huma_spec_test?mode=memory&cache=shared", db.Pool{})
	if err != nil {
		t.Fatalf("open memdb: %v", err)
	}
	if err := db.Migrate(dbs, d); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { dbs.Close() })
	return dbs, d
}

type fakeAccountStore struct{}

func (fakeAccountStore) FindForLogin(context.Context, string) (*auth.LoginAccount, error) {
	return nil, auth.ErrAccountNotFound
}
func (fakeAccountStore) Authorize(context.Context, int) (*auth.AuthInfo, error) {
	return &auth.AuthInfo{Enabled: true}, nil
}
func (fakeAccountStore) ChangePassword(context.Context, int, string, string) error { return nil }

func TestHumaDocsAndSystemConfig(t *testing.T) {
	info := systemconfig.NewSystemInfo("V1.0.0", "MODEL-X", "SN-1", 10)
	svc := systemconfig.NewService(systemconfig.NewMemStore(), info)
	h := New(Deps{
		SystemConfig: systemconfig.NewHandler(svc, nil),
		Stream:       stream.NewHandler(stream.NewHub(0)),
	})

	// OpenAPI spec 由 Huma 自动生成并暴露。
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/openapi.json", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "get-system-config") {
		t.Fatalf("openapi spec: status=%d body=%s", w.Code, w.Body.String())
	}

	// Scalar 文档 UI 可访问。
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/docs", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("docs UI: status=%d", w.Code)
	}

	// GET 是公开接口，响应信封结构必须与旧版一致：{code,msg,data:{...}}。
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/system-config", nil))
	body := w.Body.String()
	if w.Code != http.StatusOK || !strings.Contains(body, `"code":0`) || !strings.Contains(body, `"data":{`) || !strings.Contains(body, `"product_name"`) {
		t.Fatalf("GET system-config envelope mismatch: status=%d body=%s", w.Code, body)
	}
}

func TestHumaSpecCoversAllOperations(t *testing.T) {
	info := systemconfig.NewSystemInfo("V1.0.0", "MODEL-X", "SN-1", 10)
	svc := systemconfig.NewService(systemconfig.NewMemStore(), info)
	authn := auth.New("test-secret", 2*time.Hour, fakeAccountStore{})
	sqldb, dialect := newTestDB(t)
	h := New(Deps{
		Authn:        authn,
		Admin:        identity.NewAdminHandler(identity.NewAdminRepo(sqldb, dialect)),
		Media:        media.NewHandler(media.NewService(media.NewMemStore(), nil)),
		Metrics:      metrics.NewHandler(metrics.NewService(metrics.NewCollector(), nil, time.Second, 10)),
		SystemConfig: systemconfig.NewHandler(svc, nil),
		Stream:       stream.NewHandler(stream.NewHub(0)),
	})

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/openapi.json", nil))
	body := w.Body.String()
	for _, op := range []string{
		"get-system-config", "update-system-config", "upload-system-image",
		"login", "change-password",
		"list-channels", "create-channel", "update-channel", "delete-channel",
		"get-metrics-snapshot", "publish-event",
	} {
		if !strings.Contains(body, op) {
			t.Errorf("OpenAPI spec missing operation: %s", op)
		}
	}
}

func TestHumaPartialBodyAccepted(t *testing.T) {
	// 前端 PUT 只发部分字段；Huma 不应因缺字段返回 422。
	info := systemconfig.NewSystemInfo("V1.0.0", "MODEL-X", "SN-1", 10)
	svc := systemconfig.NewService(systemconfig.NewMemStore(), info)
	h := New(Deps{SystemConfig: systemconfig.NewHandler(svc, nil)})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/api/v1/system-config", strings.NewReader(`{"product_name":"新名称"}`))
	r.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(w, r)
	if w.Code == http.StatusUnprocessableEntity {
		t.Fatalf("partial PUT rejected by Huma validation: %s", w.Body.String())
	}
}

func TestSSEStillBypassesHuma(t *testing.T) {
	h := New(Deps{Stream: stream.NewHandler(stream.NewHub(0))})

	// SSE 订阅走独立流式 mux，不能被 Huma/JSONErrors 缓冲成 JSON。
	// SSE handler 会阻塞到客户端断开，所以用可取消 context + goroutine。
	ctx, cancel := context.WithCancel(context.Background())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/stream/test-channel", nil).WithContext(ctx)
	r.Header.Set("Accept", "text/event-stream")
	done := make(chan struct{})
	go func() {
		h.ServeHTTP(w, r) // 阻塞到 cancel
		close(done)
	}()

	time.Sleep(50 * time.Millisecond) // 让 SSE handler 写出首帧与 Header
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SSE handler did not stop after context cancel")
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("SSE Content-Type = %q, want text/event-stream", ct)
	}
}

func TestHumaPathsNotDoublePrefixed(t *testing.T) {
	// servers 已声明 /api/v1，paths 里不能再出现 /api/v1，否则实际 URL 会变成 /api/v1/api/v1/...
	info := systemconfig.NewSystemInfo("V1.0.0", "MODEL-X", "SN-1", 10)
	svc := systemconfig.NewService(systemconfig.NewMemStore(), info)
	h := New(Deps{
		Media:        media.NewHandler(media.NewService(media.NewMemStore(), nil)),
		Metrics:      metrics.NewHandler(metrics.NewService(metrics.NewCollector(), nil, time.Second, 10)),
		SystemConfig: systemconfig.NewHandler(svc, nil),
		Stream:       stream.NewHandler(stream.NewHub(0)),
	})

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/openapi.json", nil))
	var spec struct {
		Paths map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &spec); err != nil {
		t.Fatalf("decode spec: %v", err)
	}
	for p := range spec.Paths {
		if strings.HasPrefix(p, "/api/v1/") {
			t.Errorf("path %q duplicates the /api/v1 server prefix", p)
		}
	}
}

func TestHumaRoutesReachable(t *testing.T) {
	// 认证关闭时直通；验证实际 URL 可访问（不是 404）。
	info := systemconfig.NewSystemInfo("V1.0.0", "MODEL-X", "SN-1", 10)
	svc := systemconfig.NewService(systemconfig.NewMemStore(), info)
	h := New(Deps{
		Media:        media.NewHandler(media.NewService(media.NewMemStore(), nil)),
		Metrics:      metrics.NewHandler(metrics.NewService(metrics.NewCollector(), nil, time.Second, 10)),
		SystemConfig: systemconfig.NewHandler(svc, nil),
		Stream:       stream.NewHandler(stream.NewHub(0)),
	})

	for _, path := range []string{"/api/v1/channels", "/api/v1/metrics", "/api/v1/system-config"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code == http.StatusNotFound {
			t.Errorf("GET %s => 404, route not mounted correctly", path)
		}
	}
}

func TestHumaPublicOperationBypassesAuth(t *testing.T) {
	// 认证开启时，公开接口（GET 系统配置 / 登录）仍不能要求 token。
	// 回归：OpenAPI 的 security:[{}] 曾被误判为需要认证，导致登录页 401。
	info := systemconfig.NewSystemInfo("V1.0.0", "MODEL-X", "SN-1", 10)
	svc := systemconfig.NewService(systemconfig.NewMemStore(), info)
	authn := auth.New("test-secret", 2*time.Hour, fakeAccountStore{})
	h := New(Deps{
		Authn:        authn,
		SystemConfig: systemconfig.NewHandler(svc, nil),
	})

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/system-config", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("public GET system-config with auth enabled: status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestHumaProtectedOperationRequiresAuth(t *testing.T) {
	info := systemconfig.NewSystemInfo("V1.0.0", "MODEL-X", "SN-1", 10)
	svc := systemconfig.NewService(systemconfig.NewMemStore(), info)
	authn := auth.New("test-secret", 2*time.Hour, fakeAccountStore{})
	h := New(Deps{
		Authn:        authn,
		SystemConfig: systemconfig.NewHandler(svc, nil),
	})

	// PUT 带 bearerAuth security，无 token 应被现有鉴权链拒绝（401）。
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/api/v1/system-config", strings.NewReader(`{"product_name":"x"}`))
	r.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("PUT system-config without token: status=%d body=%s", w.Code, w.Body.String())
	}
}
