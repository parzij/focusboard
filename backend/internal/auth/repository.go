package auth

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/parzij/focusboard/backend/internal/users"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Register(ctx context.Context, name, email, hash, sessionHash string, expiry time.Time) (users.User, error) {
	var user users.User
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `INSERT INTO users (name, email, password_hash) VALUES ($1, $2, $3)
			RETURNING id::text, name, email, created_at`, name, email, hash).
			Scan(&user.ID, &user.Name, &user.Email, &user.CreatedAt)
		var databaseError *pgconn.PgError
		if errors.As(err, &databaseError) && databaseError.Code == "23505" && databaseError.ConstraintName == "users_email_key" {
			return ErrEmailExists
		}
		if err != nil {
			return err
		}
		return insertSession(ctx, tx, user.ID, sessionHash, expiry)
	})
	return user, err
}

func (r *Repository) CredentialsByEmail(ctx context.Context, email string) (id, hash string, err error) {
	err = r.pool.QueryRow(ctx, "SELECT id::text, password_hash FROM users WHERE email = $1", email).Scan(&id, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		err = users.ErrNotFound
	}
	return
}

func (r *Repository) PasswordHash(ctx context.Context, id string) (string, error) {
	var hash string
	err := r.pool.QueryRow(ctx, "SELECT password_hash FROM users WHERE id = $1", id).Scan(&hash)
	if errors.Is(err, pgx.ErrNoRows) {
		err = users.ErrNotFound
	}
	return hash, err
}

func (r *Repository) Login(ctx context.Context, id, verifiedHash, sessionHash string, expiry time.Time) (users.User, error) {
	var user users.User
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		var currentHash string
		var err error
		user, currentHash, err = lockUser(ctx, tx, id)
		if errors.Is(err, pgx.ErrNoRows) || err == nil && currentHash != verifiedHash {
			return ErrCredentials
		}
		if err != nil {
			return err
		}
		return insertSession(ctx, tx, id, sessionHash, expiry)
	})
	return user, err
}

func (r *Repository) Rotate(ctx context.Context, oldHash, newHash string) (users.User, time.Time, error) {
	var user users.User
	var expiry time.Time
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		var id string
		err := tx.QueryRow(ctx, "SELECT user_id::text FROM refresh_tokens WHERE token_hash = $1", oldHash).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSession
		}
		if err != nil {
			return err
		}
		// Session issuance and password changes take the user row lock before writing sessions.
		user, _, err = lockUser(ctx, tx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSession
		}
		if err != nil {
			return err
		}
		err = tx.QueryRow(ctx, `UPDATE refresh_tokens SET revoked_at = clock_timestamp()
			WHERE user_id = $1 AND token_hash = $2 AND revoked_at IS NULL AND expires_at > clock_timestamp()
			RETURNING expires_at`, id, oldHash).Scan(&expiry)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSession
		}
		if err != nil {
			return err
		}
		return insertSession(ctx, tx, id, newHash, expiry)
	})
	return user, expiry, err
}

func (r *Repository) Logout(ctx context.Context, hash string) error {
	_, err := r.pool.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = clock_timestamp()
		WHERE token_hash = $1 AND revoked_at IS NULL`, hash)
	return err
}

func (r *Repository) ChangePassword(ctx context.Context, id, verifiedHash, newHash string) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		// Updating the account takes the same row lock as login and refresh.
		tag, err := tx.Exec(ctx, `UPDATE users SET password_hash = $3, updated_at = clock_timestamp()
			WHERE id = $1 AND password_hash = $2`, id, verifiedHash, newHash)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrCredentials
		}
		_, err = tx.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = clock_timestamp()
			WHERE user_id = $1 AND revoked_at IS NULL`, id)
		return err
	})
}

func lockUser(ctx context.Context, tx pgx.Tx, id string) (users.User, string, error) {
	var user users.User
	var hash string
	err := tx.QueryRow(ctx, `SELECT id::text, name, email, created_at, password_hash
		FROM users WHERE id = $1 FOR UPDATE`, id).Scan(&user.ID, &user.Name, &user.Email, &user.CreatedAt, &hash)
	return user, hash, err
}

func insertSession(ctx context.Context, tx pgx.Tx, id, hash string, expiry time.Time) error {
	_, err := tx.Exec(ctx, `INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)`, id, hash, expiry)
	return err
}
