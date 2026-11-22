package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	platformhttp "stream-hub/internal/platform/http"
)

type captureHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r.Clone())
	return nil
}

func (h *captureHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h *captureHandler) WithGroup(string) slog.Handler { return h }

func captureAccess(t *testing.T, o LoggerOptions, next http.Handler, req *http.Request) slog.Record {
	t.Helper()
	old := slog.Default()
	defer slog.SetDefault(old)

	cap := &captureHandler{}
	slog.SetDefault(slog.New(cap))
	LoggerWith(o)(next).ServeHTTP(httptest.NewRecorder(), req)

	cap.mu.Lock()
	defer cap.mu.Unlock()
	if len(cap.records) != 1 {
		t.Fatalf("want 1 access log, got %d", len(cap.records))
	}
	return cap.records[0]
}

func attrValue(r slog.Record, key string) string {
	var out string
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == key {
			out = a.Value.String()
			return false
		}
		return true
	})
	return out
}

func TestAccessLevelByStatus(t *testing.T) {
	cases := []struct {
		status int
		want   slog.Level
	}{
		{200, slog.LevelInfo},
		{404, slog.LevelWarn},
		{500, slog.LevelError},
	}
	for _, tc := range cases {
		rec := captureAccess(t, LoggerOptions{}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.status)
		}), httptest.NewRequest(http.MethodGet, "/x", nil))
		if rec.Level != tc.want {
			t.Errorf("status %d: level = %v, want %v", tc.status, rec.Level, tc.want)
		}
	}
}

func TestQuietPathDowngraded(t *testing.T) {
	rec := captureAccess(t,
		LoggerOptions{QuietPaths: []string{"/favicon.ico"}},
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) }),
		httptest.NewRequest(http.MethodGet, "/favicon.ico", nil))
	if rec.Level != slog.LevelDebug {
		t.Errorf("favicon 404 should be DEBUG, got %v", rec.Level)
	}
}

func TestSlowRequestUpgraded(t *testing.T) {
	rec := captureAccess(t,
		LoggerOptions{SlowThreshold: time.Microsecond},
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(2 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}),
		httptest.NewRequest(http.MethodGet, "/slow", nil))
	if rec.Level != slog.LevelWarn {
		t.Errorf("slow request should be WARN, got %v (duration=%s)", rec.Level, attrValue(rec, "duration"))
	}
}

func TestRouteTemplateInLog(t *testing.T) {
	r := platformhttp.New(nil)
	r.Public("GET /api/v1/users/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	rec := captureAccess(t, LoggerOptions{}, r.Handler(), httptest.NewRequest(http.MethodGet, "/api/v1/users/42", nil))
	if got := attrValue(rec, "route"); got != "/api/v1/users/{id}" {
		t.Errorf("route = %q, want template without method prefix", got)
	}
}
