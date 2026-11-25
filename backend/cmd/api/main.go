package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	address := os.Getenv("HTTP_ADDR")
	if address == "" {
		address = "127.0.0.1:8080"
	}

	server := &http.Server{
		Addr:              address,
		Handler:           chi.NewRouter(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       time.Minute,
	}

	if err := serve(ctx, server); err != nil {
		slog.Error("API server stopped", "error", err)
		os.Exit(1)
	}
}

func serve(ctx context.Context, server *http.Server) error {
	if ctx.Err() != nil {
		return nil
	}

	stopped := make(chan error, 1)
	go func() {
		slog.Info("Starting API server", "address", server.Addr)
		stopped <- server.ListenAndServe()
	}()

	var err error
	select {
	case err = <-stopped:
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		err = <-stopped
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
