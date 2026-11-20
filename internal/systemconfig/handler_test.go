package systemconfig

import (
	"net/http"
	"net/http/httptest"
	"strings"
)

func newRecorder() *httptest.ResponseRecorder {
	return httptest.NewRecorder()
}

func newGET() *http.Request {
	return httptest.NewRequest(http.MethodGet, "/api/v1/system-config", nil)
}

func newPUT(body string) *http.Request {
	return httptest.NewRequest(http.MethodPut, "/api/v1/system-config", strings.NewReader(body))
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}
