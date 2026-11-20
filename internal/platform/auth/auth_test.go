// auth 包测试：验签核心 + 各中间件的 HTTP 行为。
// 用库自造"坏 token"（过期/错密钥/alg=none/篡改）逐条验证被拒；
// RequirePermission / EnforcePasswordChange 用一个 fake AccountStore 覆盖，无需真库。
package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "0123456789abcdef"

func newTestAuth(ttl time.Duration) *Authenticator {
	return New(testSecret, ttl, nil) // 纯 JWT 场景不需要账号库
}

// TestIssueVerifyRoundTrip 签发→验证，claims 要原样回来（sub + mustChange）。
func TestIssueVerifyRoundTrip(t *testing.T) {
	a := newTestAuth(time.Hour)
	token, err := a.Issue("42", true)
	if err != nil {
		t.Fatalf("Issue 失败: %v", err)
	}
	claims, err := a.Verify(token)
	if err != nil {
		t.Fatalf("Verify 失败: %v", err)
	}
	if claims.Subject != "42" || !claims.MustChange {
		t.Errorf("claims 不符: %+v", claims)
	}
}

// TestVerifyRejects 表驱动：各种坏 token 都要被拒。
func TestVerifyRejects(t *testing.T) {
	a := newTestAuth(time.Hour)
	good, _ := a.Issue("1", false)

	// 篡改 payload：解出 claims、改 sub、重新编码、保留原 header+signature。
	tampered := tamperToken(t, good)

	// 手工造一个 alg=none 的攻击 token（用库以外的方式拼三段）。
	noneHeader := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"hacker","exp":9999999999}`))

	// 过期：同密钥合法签一个 exp 在过去的 token（只为测过期分支）。
	expired := signClaims(a, Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	})

	// 别的密钥签的 token（模拟密钥不匹配）。
	other := New("wrong-secret-wrong-secret", time.Hour, nil)
	foreign, _ := other.Issue("2", false)

	tests := []struct {
		name  string
		token string
		want  error
	}{
		{"格式不对", "not-a-jwt", ErrTokenInvalid},
		{"两段缺签名", strings.Join(strings.Split(good, ".")[:2], "."), ErrTokenInvalid},
		{"payload 被篡改", tampered, ErrTokenInvalid},
		{"alg=none 攻击", noneHeader + "." + payload + ".", ErrTokenInvalid},
		{"签名有效但已过期", expired, ErrTokenExpired},
		{"别的密钥签的", foreign, ErrTokenInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := a.Verify(tt.token)
			if err != tt.want {
				t.Errorf("期望 %v，实际 %v", tt.want, err)
			}
		})
	}
}

// tamperToken 篡改 payload 段：保持 header 和原 signature，中间改了 sub 就废。
func tamperToken(t *testing.T, token string) string {
	t.Helper()
	parts := strings.Split(token, ".")
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("解 payload 失败: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("解 json 失败: %v", err)
	}
	m["sub"] = "999"
	fake, _ := json.Marshal(m)
	parts[1] = base64.RawURLEncoding.EncodeToString(fake)
	return strings.Join(parts, ".")
}

// signClaims 测试专用后门：用 Authenticator 自己的密钥签一个手工指定的 claims。
func signClaims(a *Authenticator, c Claims) string {
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(a.secret)
	if err != nil {
		panic(err) // 测试辅助，出错就是致命
	}
	return s
}

// TestMiddleware 用 httptest 对认证关卡（401 档）做请求级验证。
func TestMiddleware(t *testing.T) {
	a := newTestAuth(time.Hour)
	ok := a.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	expiredToken := signClaims(a, Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	})

	tests := []struct {
		name       string
		authHeader string
		wantStatus int
	}{
		{"没带头 → 401", "", http.StatusUnauthorized},
		{"方案名不是 Bearer → 401", "Basic abc", http.StatusUnauthorized},
		{"好 token → 200", "Bearer " + mustToken(t, a, "1", false), http.StatusOK},
		{"过期 token → 401", "Bearer " + expiredToken, http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/anything", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rec := httptest.NewRecorder()
			ok.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Errorf("期望 HTTP %d，实际 %d", tt.wantStatus, rec.Code)
			}
		})
	}
}

// fakeAccounts 是内存版 AccountStore，供 RequirePermission / EnforcePasswordChange 测试。
type fakeAccounts struct {
	infos map[int]*AuthInfo
}

func (f *fakeAccounts) FindForLogin(context.Context, string) (*LoginAccount, error) {
	panic("unused in these tests")
}
func (f *fakeAccounts) Authorize(_ context.Context, uid int) (*AuthInfo, error) {
	if info, ok := f.infos[uid]; ok {
		return info, nil
	}
	return &AuthInfo{Permissions: map[string]bool{}}, nil
}
func (f *fakeAccounts) ChangePassword(context.Context, int, string, string) error {
	panic("unused in these tests")
}

// TestRequirePermission 覆盖：有权限→200、缺权限→403、账号停用→403。
func TestRequirePermission(t *testing.T) {
	accounts := &fakeAccounts{infos: map[int]*AuthInfo{
		1: {Enabled: true, Permissions: map[string]bool{PermUserDelete: true}},
		2: {Enabled: true, Permissions: map[string]bool{}},
		3: {Enabled: false, Permissions: map[string]bool{PermUserDelete: true}},
	}}
	a := New(testSecret, time.Hour, accounts)
	handler := a.Middleware(a.RequirePermission(PermUserDelete)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }),
	))

	tests := []struct {
		name       string
		uid        string
		wantStatus int
	}{
		{"有删除权限 → 200", "1", http.StatusOK},
		{"无删除权限 → 403", "2", http.StatusForbidden},
		{"账号停用 → 403", "3", http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, "/users/9", nil)
			req.Header.Set("Authorization", "Bearer "+mustToken(t, a, tt.uid, false))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Errorf("期望 HTTP %d，实际 %d", tt.wantStatus, rec.Code)
			}
		})
	}
}

// TestEnforcePasswordChange 覆盖：MustChange=true 被拦（HTTP 200 + 业务码），改过则放行。
func TestEnforcePasswordChange(t *testing.T) {
	a := newTestAuth(time.Hour)
	handler := a.Middleware(a.EnforcePasswordChange(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }),
	))

	tests := []struct {
		name       string
		mustChange bool
		wantStatus int
	}{
		{"未改密被拦 → 200(业务码)", true, http.StatusOK},
		{"已改密放行 → 200", false, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/users", nil)
			req.Header.Set("Authorization", "Bearer "+mustToken(t, a, "1", tt.mustChange))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("期望 HTTP %d，实际 %d", tt.wantStatus, rec.Code)
			}
			// 被拦时 body 应带业务码 10100；放行时是 handler 的空 200。
			var body struct {
				Code int `json:"code"`
			}
			_ = json.Unmarshal(rec.Body.Bytes(), &body)
			wantCode := CodePasswordChangeRequired
			if !tt.mustChange {
				wantCode = 0 // 放行分支 handler 没写 body，Unmarshal 失败→body.Code 仍 0
			}
			if body.Code != wantCode {
				t.Errorf("期望 code %d，实际 %d (body=%q)", wantCode, body.Code, rec.Body.String())
			}
		})
	}
}

func mustToken(t *testing.T, a *Authenticator, uid string, mustChange bool) string {
	t.Helper()
	s, err := a.Issue(uid, mustChange)
	if err != nil {
		t.Fatalf("Issue 失败: %v", err)
	}
	return s
}
