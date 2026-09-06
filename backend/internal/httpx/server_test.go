package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouterCorsUsesConfiguredOrigin(t *testing.T) {
	handler := NewRouter(Deps{AllowOrigin: "https://app.example.test"})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "https://app.example.test")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.test" {
		t.Fatalf("allow origin: got %q want configured origin", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "https://other.example.test")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("disallowed origin: got %q want empty", got)
	}
}
