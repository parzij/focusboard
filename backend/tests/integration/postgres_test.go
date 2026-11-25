package integration

import (
	"context"
	"crypto/rand"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/parzij/focusboard/backend/internal/config"
	"github.com/parzij/focusboard/backend/internal/httpx"
	"github.com/parzij/focusboard/backend/internal/platform/postgres"
	"github.com/parzij/focusboard/backend/migrations"
)

func TestPostgreSQLFoundation(t *testing.T) {
	pool, provider := testDatabase(t, 1)
	ctx := t.Context()
	t.Run("migrate up down and up again", func(t *testing.T) {
		results, err := provider.Up(ctx)
		if err != nil || len(results) != 4 {
			t.Fatalf("initial Up: %d migrations, %v", len(results), err)
		}
		assertTables(t, pool, 4)
		results, err = provider.Up(ctx)
		if err != nil || len(results) != 0 {
			t.Fatalf("idempotent Up: %d migrations, %v", len(results), err)
		}
		results, err = provider.DownTo(ctx, 0)
		if err != nil || len(results) != 4 {
			t.Fatalf("DownTo zero: %d migrations, %v", len(results), err)
		}
		assertTables(t, pool, 0)
		results, err = provider.Up(ctx)
		if err != nil || len(results) != 4 {
			t.Fatalf("second Up: %d migrations, %v", len(results), err)
		}
		assertTables(t, pool, 4)
	})
	if t.Failed() {
		return
	}

	const passwordHash = "$argon2id$v=19$m=65536,t=3,p=1$c2FsdA$aGFzaA"
	const missingID = "00000000-0000-4000-8000-000000000001"
	var userID, firstID, secondID string
	insertUser := "INSERT INTO users (name, email, password_hash) VALUES ($1, $2, $3)"
	if err := pool.QueryRow(ctx, insertUser+" RETURNING id", "Example", "board@example.com", passwordHash).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	insertTask := "INSERT INTO tasks (user_id, title, status, position) VALUES ($1, $2, $3, $4)"
	for position, id := range []*string{&firstID, &secondID} {
		if err := pool.QueryRow(ctx, insertTask+" RETURNING id", userID, "Example task", "not_started", position).Scan(id); err != nil {
			t.Fatal(err)
		}
	}
	t.Run("users", func(t *testing.T) {
		assertSQLState(t, pool, "23505", insertUser, "Another", "board@example.com", passwordHash)
		assertSQLState(t, pool, "23514", insertUser, "Another", "Board@example.com", passwordHash)
		assertSQLState(t, pool, "23514", insertUser, "Another", " board@example.com", passwordHash)
		assertSQLState(t, pool, "23514", insertUser, " \t ", "other@example.com", passwordHash)
		assertSQLState(t, pool, "23514", insertUser, "Another", "other@example.com", "plaintext")
		assertSQLState(t, pool, "23514", insertUser, "Another", "other@example.com", "$argon2id$plaintext")
	})
	t.Run("tasks and deferred ordering", func(t *testing.T) {
		assertSQLState(t, pool, "23503", insertTask, missingID, "Missing owner", "not_started", 0)
		assertSQLState(t, pool, "23514", insertTask, userID, "\t", "not_started", 2)
		assertSQLState(t, pool, "23514", insertTask, userID, "Bad status", "submitted", 2)
		assertSQLState(t, pool, "23514", insertTask, userID, "Missing position", "not_started", nil)
		assertSQLState(t, pool, "23514", insertTask, userID, "Negative position", "not_started", -1)
		assertSQLState(t, pool, "23505", insertTask, userID, "Duplicate position", "not_started", 0)
		assertSQLState(t, pool, "23514", "UPDATE tasks SET deleted_at = now() WHERE id = $1", firstID)
		assertSQLState(t, pool, "23514", "UPDATE tasks SET title = repeat('a', 121) WHERE id = $1", firstID)
		assertSQLState(t, pool, "23514", "UPDATE tasks SET description = repeat('a', 2001) WHERE id = $1", firstID)
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		for position, id := range []string{secondID, firstID} {
			if _, err := tx.Exec(ctx, "UPDATE tasks SET position = $1 WHERE id = $2", position, id); err != nil {
				t.Fatal(err)
			}
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		var due *time.Time
		if err := pool.QueryRow(ctx, "SELECT due_date FROM tasks WHERE id = $1", firstID).Scan(&due); err != nil || due != nil {
			t.Fatalf("expected nullable DATE, got %v, %v", due, err)
		}
		for _, status := range []string{"in_progress", "finish", "done"} {
			if _, err := pool.Exec(ctx, insertTask, userID, "Other column", status, 0); err != nil {
				t.Fatal(err)
			}
		}
	})
	t.Run("refresh token hashes", func(t *testing.T) {
		query := "INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, now() + interval '1 day')"
		hash := strings.Repeat("a", 64)
		if _, err := pool.Exec(ctx, query, userID, hash); err != nil {
			t.Fatal(err)
		}
		assertSQLState(t, pool, "23505", query, userID, hash)
		assertSQLState(t, pool, "23503", query, missingID, strings.Repeat("b", 64))
		assertSQLState(t, pool, "23514", query, userID, "plaintext")
	})
	t.Run("history survives soft delete and account deletion cascades", func(t *testing.T) {
		query := "INSERT INTO task_history (task_id, old_status, new_status) VALUES ($1, $2, $3)"
		assertSQLState(t, pool, "23503", query, missingID, nil, "done")
		assertSQLState(t, pool, "23514", query, firstID, "done", "done")
		assertSQLState(t, pool, "23514", query, firstID, "submitted", "done")
		assertSQLState(t, pool, "23514", query, firstID, "finish", "submitted")
		if _, err := pool.Exec(ctx, query, firstID, nil, "not_started"); err != nil {
			t.Fatal(err)
		}
		assertSQLState(t, pool, "23505", query, firstID, nil, "done")
		if _, err := pool.Exec(ctx, query, firstID, "finish", "done"); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, "UPDATE tasks SET deleted_at = now(), position = NULL WHERE id = $1", firstID); err != nil {
			t.Fatal(err)
		}
		assertCount(t, pool, 2, "SELECT count(*) FROM task_history WHERE task_id = $1", firstID)
		if _, err := pool.Exec(ctx, "DELETE FROM users WHERE id = $1", userID); err != nil {
			t.Fatal(err)
		}
		assertCount(t, pool, 0, "SELECT count(*) FROM tasks")
		assertCount(t, pool, 0, "SELECT count(*) FROM refresh_tokens")
		assertCount(t, pool, 0, "SELECT count(*) FROM task_history")
	})
	t.Run("indexes", func(t *testing.T) {
		assertCount(t, pool, 6, `SELECT count(*) FROM pg_indexes WHERE schemaname = current_schema()
			AND indexname IN ('tasks_active_board_idx', 'refresh_tokens_user_expiry_idx', 'refresh_tokens_expiry_idx',
			'task_history_task_time_idx', 'task_history_done_idx', 'task_history_initial_idx')`)
	})
	t.Run("health and readiness with a real pool deadline", func(t *testing.T) {
		router := httpx.NewRouter(func(ctx context.Context) error {
			return postgres.Check(ctx, pool, 100*time.Millisecond)
		})
		probe := func(path string, want int) {
			t.Helper()
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/"+path, nil))
			if recorder.Code != want {
				t.Fatalf("%s returned %d, expected %d", path, recorder.Code, want)
			}
		}
		probe("ready", 200)
		conn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Release()
		probe("health", 200)
		probe("ready", 503)
		conn.Release()
		probe("ready", 200)
	})
}

func testDatabase(t *testing.T, maxConnections int32) (*pgxpool.Pool, *goose.Provider) {
	t.Helper()
	dsn := os.Getenv("FOCUSBOARD_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set FOCUSBOARD_TEST_DATABASE_URL to run against a dedicated PostgreSQL test database")
	}
	settings := config.Database{
		URL: dsn, ConnectTimeout: 3 * time.Second, MaxConns: maxConnections,
		MaxConnIdle: time.Minute, MaxConnAge: time.Hour,
	}
	cfg, err := postgres.PoolConfig(settings)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ConnConfig.Database != "focusboard_test" {
		t.Fatal("integration tests require a separate database named focusboard_test")
	}
	pool, err := postgres.Open(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := postgres.Check(t.Context(), pool, 3*time.Second); err != nil {
		t.Fatal("could not connect to the integration database")
	}

	schema := "it_" + strings.ToLower(rand.Text())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := pool.Exec(t.Context(), "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := pool.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Error(err)
		}
	})

	scoped := cfg.Copy()
	scoped.ConnConfig.RuntimeParams["search_path"] = schema
	scopedPool, err := postgres.Open(t.Context(), scoped)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(scopedPool.Close)
	db := stdlib.OpenDB(*scoped.ConnConfig)
	t.Cleanup(func() { _ = db.Close() })
	provider, err := migrations.New(db)
	if err != nil {
		t.Fatal(err)
	}
	return scopedPool, provider
}

func assertSQLState(t *testing.T, pool *pgxpool.Pool, code, query string, args ...any) {
	t.Helper()
	_, err := pool.Exec(t.Context(), query, args...)
	var pgError *pgconn.PgError
	if !errors.As(err, &pgError) || pgError.Code != code {
		t.Fatalf("expected SQLSTATE %s from %s, got %v", code, query, err)
	}
}

func assertTables(t *testing.T, pool *pgxpool.Pool, want int) {
	t.Helper()
	assertCount(t, pool, want, `SELECT count(*) FROM information_schema.tables WHERE table_schema = current_schema()
		AND table_name IN ('users', 'tasks', 'refresh_tokens', 'task_history')`)
}

func assertCount(t *testing.T, pool *pgxpool.Pool, want int, query string, args ...any) {
	t.Helper()
	var got int
	err := pool.QueryRow(t.Context(), query, args...).Scan(&got)
	if err != nil || got != want {
		t.Fatalf("expected count %d, got %d (%v)", want, got, err)
	}
}
