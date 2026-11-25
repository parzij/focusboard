package app

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/parzij/focusboard/backend/internal/auth"
	"github.com/parzij/focusboard/backend/internal/config"
	"github.com/parzij/focusboard/backend/internal/httpx"
	"github.com/parzij/focusboard/backend/internal/platform/password"
	"github.com/parzij/focusboard/backend/internal/platform/postgres"
	"github.com/parzij/focusboard/backend/internal/platform/token"
	"github.com/parzij/focusboard/backend/internal/statistics"
	"github.com/parzij/focusboard/backend/internal/tasks"
	"github.com/parzij/focusboard/backend/internal/users"
)

func Router(cfg config.Config, pool *pgxpool.Pool) (http.Handler, error) {
	hasher, err := password.New(cfg.Auth.PasswordWorkers)
	if err != nil {
		return nil, err
	}
	access := token.NewAccess(cfg.Auth.Secret, cfg.Auth.Issuer, cfg.Auth.Audience, cfg.Auth.AccessTTL)
	sessions := auth.NewHandler(auth.NewService(auth.NewRepository(pool), hasher, access, cfg.Auth.RefreshTTL), cfg.Auth)
	profiles := users.NewHandler(users.NewService(users.NewRepository(pool)))
	board := tasks.NewHandler(tasks.NewService(tasks.NewRepository(pool)))

	router := httpx.NewRouter(func(ctx context.Context) error {
		return postgres.Check(ctx, pool, cfg.Database.CheckTimeout)
	})
	router.Route("/api/v1/auth", sessions.Routes)
	router.Route("/api/v1/users", func(r chi.Router) {
		r.Use(httpx.Deadline(cfg.Auth.RequestTimeout), httpx.RequireBearer(access.Verify))
		r.Get("/me", profiles.Profile)
		r.Patch("/me", profiles.Rename)
		r.With(httpx.RequireOrigin(cfg.Auth.Origin)).Put("/me/password", sessions.ChangePassword)
	})
	router.Route("/api/v1/tasks", func(r chi.Router) {
		r.Use(httpx.Deadline(cfg.Auth.RequestTimeout), httpx.RequireBearer(access.Verify))
		board.Routes(r)
	})
	stats := statistics.NewHandler(statistics.NewService(statistics.NewRepository(pool), time.Now))
	router.With(httpx.Deadline(cfg.Auth.RequestTimeout), httpx.RequireBearer(access.Verify)).Get("/api/v1/statistics", stats.Get)
	return router, nil
}
