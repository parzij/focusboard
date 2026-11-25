package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/parzij/focusboard/backend/internal/app"
	"github.com/parzij/focusboard/backend/internal/config"
	"github.com/parzij/focusboard/backend/internal/platform/postgres"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		slog.Error("API server stopped", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.LogLevel})))
	poolConfig, err := postgres.PoolConfig(cfg.Database)
	if err != nil {
		return err
	}
	pool, err := postgres.Open(ctx, poolConfig)
	if err != nil {
		return err
	}
	defer pool.Close()

	checkDatabase := func(ctx context.Context) error {
		return postgres.Check(ctx, pool, cfg.Database.CheckTimeout)
	}
	if err := checkDatabase(ctx); err != nil && ctx.Err() == nil {
		slog.Warn("PostgreSQL unavailable at startup")
	}

	router, err := app.Router(cfg, pool)
	if err != nil {
		return errors.New("could not initialize authentication")
	}
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: cfg.ReadHeader,
		ReadTimeout:       cfg.Read,
		WriteTimeout:      cfg.Write,
		IdleTimeout:       cfg.Idle,
	}
	return serve(ctx, server, cfg.Shutdown)
}

func serve(ctx context.Context, server *http.Server, shutdownTimeout time.Duration) error {
	if ctx.Err() != nil {
		return nil
	}

	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(ctx, "tcp", server.Addr)
	if err != nil {
		return err
	}
	return serveListener(ctx, server, listener, shutdownTimeout)
}

func serveListener(ctx context.Context, server *http.Server, listener net.Listener, shutdownTimeout time.Duration) error {
	defer listener.Close()
	slog.Info("Starting API server", "address", listener.Addr().String())
	stopped := make(chan error, 1)
	go func() {
		stopped <- server.Serve(listener)
	}()

	var err error
	select {
	case err = <-stopped:
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			<-stopped
			return err
		}
		err = <-stopped
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
