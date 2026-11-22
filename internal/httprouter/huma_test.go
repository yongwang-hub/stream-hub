package httprouter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"stream-hub/internal/platform/auth"
	"stream-hub/internal/stream"
	"stream-hub/internal/systemconfig"
)

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
