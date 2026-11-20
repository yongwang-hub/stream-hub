package platformhttp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeAuthn 记录各关卡是否被调用，用于验证三级注册的关卡组合。
type fakeAuthn struct {
	middleware    bool
	permission    string
	enforceChange bool
	queryToken    bool
	deny          bool
}

func (f *fakeAuthn) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.middleware = true
		if f.deny {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (f *fakeAuthn) RequirePermission(p string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			f.permission = p
			next.ServeHTTP(w, r)
		})
	}
}

func (f *fakeAuthn) EnforcePasswordChange(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.enforceChange = true
		next.ServeHTTP(w, r)
	})
}

func (f *fakeAuthn) MiddlewareQueryToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.queryToken = true
		next.ServeHTTP(w, r)
	})
}

func okHandler(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(msg)) }
}

func TestPublicSkipsAuth(t *testing.T) {
	f := &fakeAuthn{}
	r := New(f)
	r.Public("GET /public", okHandler("public"))
	w := httptest.NewRecorder()
	r.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/public", nil))
	if w.Body.String() != "public" {
		t.Fatalf("body = %q", w.Body.String())
	}
	if f.middleware {
		t.Error("public route should not run auth middleware")
	}
}

func TestNeedAuthOnlyMiddleware(t *testing.T) {
	f := &fakeAuthn{}
	r := New(f)
	r.NeedAuth("GET /me", okHandler("me"))
	w := httptest.NewRecorder()
	r.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/me", nil))
	if w.Body.String() != "me" {
		t.Fatalf("body = %q", w.Body.String())
	}
	if !f.middleware {
		t.Error("needAuth should run middleware")
	}
	if f.permission != "" || f.enforceChange {
		t.Error("needAuth should not check permission or force-password-change")
	}
}

func TestAuthorizeRunsFullChain(t *testing.T) {
	f := &fakeAuthn{}
	r := New(f)
	r.Authorize("PUT /admin", "admin:write", okHandler("admin"))
	w := httptest.NewRecorder()
	r.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/admin", nil))
	if w.Body.String() != "admin" {
		t.Fatalf("body = %q", w.Body.String())
	}
	if !f.middleware || !f.enforceChange || f.permission != "admin:write" {
		t.Errorf("authorize chain incomplete: %+v", f)
	}
}

func TestStreamUsesQueryToken(t *testing.T) {
	f := &fakeAuthn{}
	r := New(f)
	r.Stream("GET /api/v1/stream/{channel}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: hi\n\n"))
	})
	w := httptest.NewRecorder()
	r.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/stream/system-metrics", nil))
	if !f.queryToken {
		t.Error("stream should use MiddlewareQueryToken")
	}
	if f.middleware {
		t.Error("stream should not run standard bearer middleware")
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Errorf("Content-Type = %q, want event-stream (JSONErrors must be bypassed)", ct)
	}
}

func TestTypedNilAuthnBecomesNil(t *testing.T) {
	// (*fakeAuthn)(nil) 装进接口后接口非 nil，但底层指针为 nil —— 典型 typed-nil。
	var a Authenticator = (*fakeAuthn)(nil)
	if got := nonNil(a); got != nil {
		t.Errorf("nonNil should normalize typed-nil to nil, got %#v", got)
	}
	// 注册时即使传 typed-nil，也不应给公开路由挂关卡（否则调用方法会 panic）。
	r := New(a)
	r.Public("GET /x", okHandler("x"))
	w := httptest.NewRecorder()
	r.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	if w.Body.String() != "x" {
		t.Fatalf("body = %q", w.Body.String())
	}
}
