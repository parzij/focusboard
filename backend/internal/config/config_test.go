package config

import (
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	env := authEnvironment()
	for key, value := range map[string]string{
		"DATABASE_URL":      "postgres://example@localhost/example",
		"HTTP_ADDR":         "[::1]:9090",
		"LOG_LEVEL":         "warn",
		"DB_POOL_MAX_CONNS": "8",
		"DB_POOL_MIN_CONNS": "2",
		"DB_CHECK_TIMEOUT":  "350ms",
	} {
		env[key] = value
	}
	got, err := load(func(key string) string { return env[key] })
	if err != nil {
		t.Fatal(err)
	}
	if got.HTTPAddr != "[::1]:9090" || got.LogLevel != slog.LevelWarn || got.ReadHeader != 5*time.Second ||
		got.Database.MaxConns != 8 || got.Database.MinConns != 2 || got.Database.CheckTimeout != 350*time.Millisecond {
		t.Fatal("overrides or defaults were lost")
	}
}

func TestLoadRejectsInvalidSettings(t *testing.T) {
	const sensitive = "sensitive-example"
	for _, test := range []struct{ key, value string }{
		{"DATABASE_URL", ""},
		{"HTTP_ADDR", sensitive},
		{"HTTP_ADDR", ":65536"},
		{"HTTP_ADDR", ":0"},
		{"LOG_LEVEL", sensitive},
		{"HTTP_READ_TIMEOUT", "0s"},
		{"HTTP_READ_HEADER_TIMEOUT", "11s"},
		{"HTTP_IDLE_TIMEOUT", "-1s"},
		{"HTTP_SHUTDOWN_TIMEOUT", sensitive},
		{"DB_CHECK_TIMEOUT", "15s"},
		{"DB_CONNECT_TIMEOUT", sensitive},
		{"DB_MAX_CONN_LIFETIME", "0s"},
		{"DB_POOL_MAX_CONNS", "0"},
		{"DB_POOL_MAX_CONNS", "2147483648"},
		{"DB_POOL_MIN_CONNS", "11"},
		{"DB_POOL_MIN_CONNS", "-1"},
	} {
		t.Run(test.key+"/"+test.value, func(t *testing.T) {
			env := authEnvironment()
			env["DATABASE_URL"] = "postgres://example@localhost/example"
			env[test.key] = test.value
			_, err := load(func(key string) string { return env[key] })
			if err == nil || !strings.Contains(err.Error(), test.key) || strings.Contains(err.Error(), sensitive) {
				t.Fatalf("expected a redacted error for %s", test.key)
			}
		})
	}
}
