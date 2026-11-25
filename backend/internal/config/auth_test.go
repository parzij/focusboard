package config

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func authEnvironment() map[string]string {
	return map[string]string{
		"JWT_SECRET": base64.StdEncoding.EncodeToString([]byte("abcdefghijklmnopqrstuvwxyz012345")),
		"JWT_ISSUER": "test-issuer", "JWT_AUDIENCE": "test-audience",
		"APP_ENV": "production", "APP_ORIGIN": "https://focusboard.example.com",
	}
}

func TestAuthDefaults(t *testing.T) {
	env := authEnvironment()
	cfg, err := loadAuth(func(key string) string { return env[key] }, 15*time.Second)
	if err != nil || len(cfg.Secret) != 32 || !cfg.CookieSecure || cfg.AccessTTL != 15*time.Minute ||
		cfg.RefreshTTL != 30*24*time.Hour || cfg.PasswordWorkers != 2 {
		t.Fatalf("invalid defaults: %v", err)
	}
	env["APP_ENV"], env["APP_ORIGIN"], env["COOKIE_SECURE"] = "development", "http://127.0.0.1:8080", "false"
	if _, err := loadAuth(func(key string) string { return env[key] }, 15*time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestAuthRejectsUnsafeSettings(t *testing.T) {
	for _, test := range []struct{ key, value string }{
		{"JWT_SECRET", ""}, {"JWT_SECRET", "sensitive-example"},
		{"JWT_SECRET", base64.StdEncoding.EncodeToString(make([]byte, 32))},
		{"JWT_ISSUER", ""}, {"JWT_AUDIENCE", " "},
		{"APP_ORIGIN", "https://focusboard.example.com/"}, {"APP_ORIGIN", "http://localhost:8080"},
		{"APP_ORIGIN", "https://user:sensitive-example@focusboard.example.com"},
		{"COOKIE_SECURE", "false"}, {"COOKIE_SECURE", "maybe"}, {"REFRESH_COOKIE_NAME", "__Host-test"},
		{"ACCESS_TOKEN_TTL", "16m"}, {"REFRESH_TOKEN_TTL", "721h"},
		{"AUTH_REQUEST_TIMEOUT", "15s"}, {"AUTH_RATE_WINDOW", "0s"},
		{"AUTH_RATE_MAX_KEYS", "0"}, {"AUTH_PASSWORD_WORKERS", "9"},
	} {
		t.Run(test.key+"/"+test.value, func(t *testing.T) {
			env := authEnvironment()
			env[test.key] = test.value
			_, err := loadAuth(func(key string) string { return env[key] }, 15*time.Second)
			if err == nil || !strings.Contains(err.Error(), test.key) || strings.Contains(err.Error(), "sensitive-example") {
				t.Fatalf("expected redacted configuration error for %s, got %v", test.key, err)
			}
		})
	}
}
