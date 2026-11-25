package httpx

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type errorEnvelope struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewRouter(checkDatabase func(context.Context) error) http.Handler {
	router := chi.NewRouter()
	router.Use(logRequest)
	router.Get("/api/v1/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	router.Get("/api/v1/ready", func(w http.ResponseWriter, r *http.Request) {
		if err := checkDatabase(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, errorEnvelope{apiError{
				Code: "SERVICE_UNAVAILABLE", Message: "Service temporarily unavailable.",
			}})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	return router
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := rand.Text()
		response := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		response.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(response, r)
		route := chi.RouteContext(r.Context()).RoutePattern()
		slog.InfoContext(r.Context(), "HTTP request", "request_id", requestID,
			"method", r.Method, "route", route, "status", response.Status(), "duration", time.Since(start))
	})
}
