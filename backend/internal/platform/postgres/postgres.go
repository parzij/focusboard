package postgres

import (
	"context"
	"errors"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/parzij/focusboard/backend/internal/config"
)

func PoolConfig(settings config.Database) (*pgxpool.Config, error) {
	dsn, err := url.Parse(settings.URL)
	if err != nil || (dsn.Scheme != "postgres" && dsn.Scheme != "postgresql") || dsn.Host == "" {
		return nil, errors.New("DATABASE_URL must be a PostgreSQL URL with a host")
	}

	cfg, err := pgxpool.ParseConfig(settings.URL)
	if err != nil {
		return nil, errors.New("DATABASE_URL contains invalid PostgreSQL connection settings")
	}

	cfg.ConnConfig.ConnectTimeout = settings.ConnectTimeout
	cfg.MaxConns = settings.MaxConns
	cfg.MinConns = settings.MinConns
	cfg.MaxConnIdleTime = settings.MaxConnIdle
	cfg.MaxConnLifetime = settings.MaxConnAge
	return cfg, nil
}

func Open(ctx context.Context, cfg *pgxpool.Config) (*pgxpool.Pool, error) {
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, errors.New("could not initialize the PostgreSQL pool")
	}
	return pool, nil
}

func Check(ctx context.Context, pool *pgxpool.Pool, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return pool.Ping(ctx)
}
