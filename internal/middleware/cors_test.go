package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSPreflight(t *testing.T) {
	called := false
	h := CORS([]string{"http://localhost:5173"}, false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	r := httptest.NewRequest(http.MethodOptions, "/api/v1/channels", nil)
	r.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("预检应返回 204，实际 %d", w.Code)
	}
	if called {
		t.Error("预检不应进入业务 handler")
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("Allow-Origin = %q", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Error("预检缺少 Allow-Methods")
	}
}

func TestCORSAllowedOriginAndRejectedOrigin(t *testing.T) {
	h := CORS([]string{"http://localhost:5173"}, false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	for _, tc := range []struct {
		origin     string
		wantHeader string
		wantStatus int
	}{
		{"http://localhost:5173", "http://localhost:5173", http.StatusOK},
		{"https://evil.example", "", http.StatusOK},
	} {
		r := httptest.NewRequest(http.MethodGet, "/ping", nil)
		r.Header.Set("Origin", tc.origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != tc.wantHeader {
			t.Errorf("origin %q: Allow-Origin = %q, want %q", tc.origin, got, tc.wantHeader)
		}
		if w.Code != tc.wantStatus {
			t.Errorf("origin %q: status = %d", tc.origin, w.Code)
		}
	}
}

func TestCORSCredentialsNeverCombineWithWildcard(t *testing.T) {
	h := CORS([]string{"*"}, true)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	r := httptest.NewRequest(http.MethodGet, "/ping", nil)
	r.Header.Set("Origin", "https://evil.example")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("credentials + '*' 时不应回 Allow-Origin，实际 %q", got)
	}
}
