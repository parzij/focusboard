package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/parzij/focusboard/backend/internal/config"
	"github.com/parzij/focusboard/backend/internal/platform/postgres"
	"github.com/parzij/focusboard/backend/migrations"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if len(os.Args) != 2 {
		slog.Error("Usage: migrate up|down|status")
		os.Exit(1)
	}
	if err := run(ctx, os.Args[1]); err != nil {
		slog.Error("Migration command failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, command string) error {
	if command != "up" && command != "down" && command != "status" {
		return errors.New("expected up, down, or status")
	}
	settings, err := config.LoadDatabase()
	if err != nil {
		return err
	}
	cfg, err := postgres.PoolConfig(settings)
	if err != nil {
		return err
	}
	db := stdlib.OpenDB(*cfg.ConnConfig)
	defer db.Close()

	pingCtx, cancel := context.WithTimeout(ctx, settings.ConnectTimeout)
	err = db.PingContext(pingCtx)
	cancel()
	if err != nil {
		return errors.New("could not reach PostgreSQL for migrations")
	}

	provider, err := migrations.New(db)
	if err != nil {
		return errors.New("could not initialize Goose")
	}
	ctx, cancel = context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	switch command {
	case "up":
		results, err := provider.Up(ctx)
		if err != nil {
			return migrationError("Goose could not apply migrations", err)
		}
		slog.Info("Migrations up to date", "applied", len(results))
	case "down":
		result, err := provider.Down(ctx)
		if err != nil {
			return migrationError("Goose could not roll back the latest migration", err)
		}
		slog.Info("Migration rolled back", "version", result.Source.Version)
	case "status":
		status, err := provider.Status(ctx)
		if err != nil {
			return migrationError("Goose could not read migration status", err)
		}
		for _, migration := range status {
			fmt.Printf("%05d %s\n", migration.Source.Version, migration.State)
		}
	}
	return nil
}

func migrationError(message string, err error) error {
	var partial *goose.PartialError
	if errors.As(err, &partial) {
		message = fmt.Sprintf("%s (version %d)", message, partial.Failed.Source.Version)
	}
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) {
		message = fmt.Sprintf("%s (SQLSTATE %s)", message, databaseError.Code)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		message += " (deadline exceeded)"
	}
	return errors.New(message)
}
