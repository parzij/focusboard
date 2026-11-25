package postgres

import (
	"strings"
	"testing"
	"time"

	"github.com/parzij/focusboard/backend/internal/config"
)

func TestPoolConfig(t *testing.T) {
	cfg, err := PoolConfig(config.Database{
		URL:            "postgres://example@localhost/example?connect_timeout=20",
		ConnectTimeout: 2 * time.Second,
		MaxConns:       8,
		MinConns:       2,
		MaxConnIdle:    time.Minute,
		MaxConnAge:     time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ConnConfig.ConnectTimeout != 2*time.Second || cfg.MaxConns != 8 || cfg.MinConns != 2 ||
		cfg.MaxConnIdleTime != time.Minute || cfg.MaxConnLifetime != time.Hour {
		t.Fatal("database settings were lost")
	}
}

func TestPoolConfigRedactsInvalidURLs(t *testing.T) {
	for _, dsn := range []string{
		"mysql://example:sensitive@localhost/example",
		"postgres://example:sensitive@localhost/example?sslmode=invalid",
		"postgres://example:sensitive@localhost:invalid/example",
	} {
		_, err := PoolConfig(config.Database{URL: dsn})
		if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") || strings.Contains(err.Error(), "sensitive") {
			t.Fatal("expected a redacted DATABASE_URL error")
		}
	}
}
