package auth

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/parzij/focusboard/backend/internal/config"
)

func TestLimiterBoundsConcurrencyAndExpires(t *testing.T) {
	l := newLimiter(time.Minute, 2)
	now := time.Unix(1000, 0)
	l.now = func() time.Time { return now }
	var allowed atomic.Int32
	var group sync.WaitGroup
	for range 50 {
		group.Go(func() {
			if ok, _ := l.allow("account:a", 5); ok {
				allowed.Add(1)
			}
		})
	}
	group.Wait()
	if allowed.Load() != 5 {
		t.Fatalf("allowed %d", allowed.Load())
	}
	if ok, _ := l.allow("account:b", 1); !ok {
		t.Fatal("second key rejected")
	}
	if ok, retry := l.allow("account:c", 1); ok || retry != time.Minute {
		t.Fatal("key cap was not applied")
	}
	if len(l.counts) != 2 {
		t.Fatal("expired map exceeded the cap")
	}
	now = now.Add(time.Minute)
	if ok, _ := l.allow("account:c", 1); !ok || len(l.counts) != 1 {
		t.Fatal("window did not reset")
	}
}

func TestIPLimitIgnoresForwardedHeaders(t *testing.T) {
	h := NewHandler(nil, config.Auth{RateWindow: time.Minute, RateKeys: 20, IPLimit: 1})
	wrapped := h.limitIP(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	for i, request := range []struct {
		peer, forwarded string
		status          int
	}{
		{"192.0.2.4:4162", "192.0.2.9", 204}, {"[::ffff:192.0.2.4]:4578", "192.0.2.10", 429},
		{"[2001:db8::2]:4162", "192.0.2.9", 204}, {"[2001:db8::3]:4578", "192.0.2.10", 429},
	} {
		recorder := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/", nil)
		r.RemoteAddr = request.peer
		r.Header.Set("X-Forwarded-For", request.forwarded)
		r.Header.Set("Forwarded", "for="+request.forwarded)
		wrapped.ServeHTTP(recorder, r)
		if recorder.Code != request.status || request.status == 429 && recorder.Header().Get("Retry-After") != "60" {
			t.Fatalf("request %d: status %d, Retry-After %s", i, recorder.Code, recorder.Header().Get("Retry-After"))
		}
	}
}
