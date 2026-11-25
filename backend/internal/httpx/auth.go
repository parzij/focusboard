package httpx

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type apiError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func WriteError(w http.ResponseWriter, status int, code, message string, fields map[string]string) {
	WriteJSON(w, status, struct {
		Error apiError `json:"error"`
	}{apiError{Code: code, Message: message, Fields: fields}})
}

func ValidationError(w http.ResponseWriter, fields map[string]string) {
	WriteError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Please check the entered data.", fields)
}

func Unauthorized(w http.ResponseWriter) {
	WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required.", nil)
}

func InternalError(w http.ResponseWriter, r *http.Request, operation string, err error) {
	// Error types are useful for diagnosis. Driver error strings may contain data or a DSN.
	slog.ErrorContext(r.Context(), "Request failed", "request_id", w.Header().Get("X-Request-ID"),
		"operation", operation, "error_type", errorType(err))
	if errors.Is(err, context.DeadlineExceeded) {
		WriteError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Service temporarily unavailable.", nil)
		return
	}
	WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.", nil)
}

func errorType(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline"
	case errors.Is(err, context.Canceled):
		return "canceled"
	default:
		return "internal"
	}
}

func ReadJSON[T any](w http.ResponseWriter, r *http.Request) (T, bool) {
	var input *T
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := json.UnmarshalRead(r.Body, &input, json.RejectUnknownMembers(true)); err == nil && input != nil {
		return *input, true
	}
	WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Expected a single valid JSON object.", nil)
	var zero T
	return zero, false
}

func EmptyBody(w http.ResponseWriter, r *http.Request) bool {
	var probe [1]byte
	if n, err := io.ReadFull(r.Body, probe[:]); n == 0 && err == io.EOF {
		return true
	}
	WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Expected an empty request body.", nil)
	return false
}

func RequireOrigin(expected string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origins := r.Header.Values("Origin")
			if len(origins) != 1 || origins[0] != expected {
				WriteError(w, http.StatusForbidden, "FORBIDDEN", "Origin is not allowed.", nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func Deadline(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type userKey struct{}

func UserID(ctx context.Context) string {
	value, _ := ctx.Value(userKey{}).(string)
	return value
}

func RequireBearer(verify func(string) (string, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			headers := r.Header.Values("Authorization")
			if len(headers) == 1 {
				scheme, value, ok := strings.Cut(headers[0], " ")
				if ok && strings.EqualFold(scheme, "Bearer") && value != "" {
					if id, err := verify(value); err == nil {
						next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey{}, id)))
						return
					}
				}
			}
			w.Header().Set("WWW-Authenticate", "Bearer")
			Unauthorized(w)
		})
	}
}

func NoContent(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}
