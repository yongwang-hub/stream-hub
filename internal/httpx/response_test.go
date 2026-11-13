package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func decodeEnvelope(t *testing.T, body []byte) Response {
	t.Helper()
	var resp Response
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("响应不是 JSON 信封: %v (%s)", err, body)
	}
	return resp
}

func TestJSONErrorsConverts404(t *testing.T) {
	h := JSONErrors(http.NewServeMux())
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d", w.Code)
	}
	if got := decodeEnvelope(t, w.Body.Bytes()).Code; got != CodeSystemNotFound {
		t.Errorf("code = %d, want %d", got, CodeSystemNotFound)
	}
}

func TestJSONErrorsConverts405AndKeepsAllow(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /only", func(w http.ResponseWriter, r *http.Request) { OK(w, "ok") })
	h := JSONErrors(mux)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/only", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", w.Code)
	}
	if got := decodeEnvelope(t, w.Body.Bytes()).Code; got != CodeSystemMethod {
		t.Errorf("code = %d, want %d", got, CodeSystemMethod)
	}
	if w.Header().Get("Allow") == "" {
		t.Error("405 应保留 Allow 头")
	}
}

func TestJSONErrorsPassesThroughNormalResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ok", func(w http.ResponseWriter, r *http.Request) { OK(w, "pong") })
	h := JSONErrors(mux)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ok", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	resp := decodeEnvelope(t, w.Body.Bytes())
	if resp.Code != CodeOK || resp.Data != "pong" {
		t.Errorf("正常响应被改写: %+v", resp)
	}
}
