package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestLimitRejectsOverflowAndReleasesSlot(t *testing.T) {
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	var once sync.Once
	h := Limit(1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		once.Do(func() { entered <- struct{}{} })
		<-release
		w.WriteHeader(http.StatusOK)
	}))

	done := make(chan struct{})
	go func() {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
		close(done)
	}()
	<-entered

	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/", nil))
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("超额请求应 429，实际 %d", w2.Code)
	}
	close(release)
	<-done // 等第一个请求真正返回、slot 归还

	// slot 归还后，新请求应能正常通过（不再阻塞）。
	w3 := httptest.NewRecorder()
	h.ServeHTTP(w3, httptest.NewRequest(http.MethodGet, "/", nil))
	if w3.Code != http.StatusOK {
		t.Errorf("slot 释放后应 200，实际 %d", w3.Code)
	}
}

func TestLimitZeroIsPassthrough(t *testing.T) {
	h := Limit(0)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusCreated {
		t.Errorf("max<=0 应零开销透传，实际 %d", w.Code)
	}
}

func TestLimitBypassSkipsAdmission(t *testing.T) {
	// max=1，但 bypass 的请求不占 slot：即使第一个请求占着额度，被豁免的请求仍能通过。
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	h := Limit(1, func(r *http.Request) bool {
		return r.URL.Path == "/api/v1/stream/room"
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/stream/room" {
			w.WriteHeader(http.StatusOK)
			return
		}
		entered <- struct{}{}
		<-release
		w.WriteHeader(http.StatusOK)
	}))

	done := make(chan struct{})
	go func() {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/normal", nil))
		close(done)
	}()
	<-entered

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/stream/room", nil))
	if w.Code != http.StatusOK {
		t.Errorf("SSE 长连接应被豁免，实际 %d", w.Code)
	}
	close(release)
	<-done
}
