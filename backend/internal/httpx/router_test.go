package httpx

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthAndReadiness(t *testing.T) {
	for _, test := range []struct {
		name    string
		path    string
		dbError error
		status  int
		body    string
	}{
		{"health with database failure", "/api/v1/health", errors.New("internal connection details"), 200, `{"status":"ok"}`},
		{"ready", "/api/v1/ready", nil, 200, `{"status":"ready"}`},
		{"unavailable", "/api/v1/ready", errors.New("internal connection details"), 503, `{"error":{"code":"SERVICE_UNAVAILABLE","message":"Service temporarily unavailable."}}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			type contextKey struct{}
			requestCtx := context.WithValue(t.Context(), contextKey{}, test.name)
			request := httptest.NewRequestWithContext(requestCtx, http.MethodGet, test.path, nil)
			result := httptest.NewRecorder()
			var called bool
			NewRouter(func(ctx context.Context) error {
				called = true
				if ctx.Value(contextKey{}) != test.name {
					t.Error("database check did not receive the request context")
				}
				return test.dbError
			}).ServeHTTP(result, request)
			if called != (test.path == "/api/v1/ready") || result.Code != test.status ||
				result.Header().Get("Content-Type") != "application/json" ||
				result.Header().Get("X-Request-ID") == "" ||
				result.Header().Get("Cache-Control") != "no-store" || result.Body.String() != test.body+"\n" {
				t.Fatalf("unexpected probe response: %d %s", result.Code, result.Body)
			}
		})
	}
}
