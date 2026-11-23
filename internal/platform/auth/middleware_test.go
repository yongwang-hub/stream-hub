package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type stubAccounts struct{ info *AuthInfo }

func (s *stubAccounts) FindForLogin(context.Context, string) (*LoginAccount, error) {
	return nil, ErrAccountNotFound
}
func (s *stubAccounts) Authorize(context.Context, int) (*AuthInfo, error)         { return s.info, nil }
func (s *stubAccounts) ChangePassword(context.Context, int, string, string) error { return nil }

func TestOldTokenInvalidatedAfterPasswordChange(t *testing.T) {
	accounts := &stubAccounts{info: &AuthInfo{
		Enabled:     true,
		Permissions: map[string]bool{PermUserList: true},
	}}
	a := New("test-secret", time.Hour, accounts)
	token, err := a.Issue("1", false)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	h := a.Middleware(a.RequirePermission(PermUserList)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))
	do := func() int {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/users", nil)
		r.Header.Set("Authorization", "Bearer "+token)
		h.ServeHTTP(w, r)
		return w.Code
	}

	// 未改密：token 有效。
	accounts.info.PasswordChangedAt = time.Time{}
	if got := do(); got != http.StatusOK {
		t.Fatalf("未改密时应放行，实际 %d", got)
	}
	// 改密时间晚于 token 的 iat：旧 token 立即失效。
	accounts.info.PasswordChangedAt = time.Now().Add(time.Hour)
	if got := do(); got != http.StatusUnauthorized {
		t.Errorf("改密后旧 token 应 401，实际 %d", got)
	}
}

func TestTokenIssuedSameSecondAsPasswordChangeStaysValid(t *testing.T) {
	accounts := &stubAccounts{info: &AuthInfo{
		Enabled:     true,
		Permissions: map[string]bool{PermUserList: true},
	}}
	a := New("test-secret", time.Hour, accounts)
	token, err := a.Issue("1", false)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	// 改密响应里新签发的 token：JWT iat 截断到秒，库时间带纳秒且更晚。
	accounts.info.PasswordChangedAt = time.Now().Truncate(time.Second).Add(500 * time.Millisecond)

	h := a.Middleware(a.RequirePermission(PermUserList)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/users", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("同秒签发的新 token 不应被纳秒时间戳误判失效，实际 %d: %s", w.Code, w.Body.String())
	}
}

func TestMiddlewareQueryTokenAcceptsQueryParam(t *testing.T) {
	a := New("test-secret", time.Hour, nil)
	token, _ := a.Issue("7", false)
	h := a.MiddlewareQueryToken(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if FromContext(r.Context()) == nil {
			t.Error("claims 未注入 context")
		}
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/stream/room?token="+token, nil))
	if w.Code != http.StatusOK {
		t.Errorf("query token 应放行，实际 %d", w.Code)
	}
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/api/v1/stream/room", nil))
	if w2.Code != http.StatusUnauthorized {
		t.Errorf("缺 token 应 401，实际 %d", w2.Code)
	}
}
