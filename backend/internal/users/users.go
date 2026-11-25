package users

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID        string
	Name      string
	Email     string
	CreatedAt time.Time
}

var (
	ErrNotFound    = errors.New("user not found")
	ErrInvalidName = errors.New("invalid name")
)

const NameError = "Name must contain 1–80 characters."

func NormalizeName(value string) (string, error) {
	value = strings.TrimSpace(value)
	count := utf8.RuneCountInString(value)
	if count < 1 || count > 80 || strings.ContainsRune(value, 0) {
		return "", ErrInvalidName
	}
	return value, nil
}

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Profile(ctx context.Context, id string) (User, error) {
	return scan(r.pool.QueryRow(ctx, "SELECT id::text, name, email, created_at FROM users WHERE id = $1", id))
}

func (r *Repository) Rename(ctx context.Context, id, name string) (User, error) {
	return scan(r.pool.QueryRow(ctx, `UPDATE users SET name = $2, updated_at = clock_timestamp() WHERE id = $1
		RETURNING id::text, name, email, created_at`, id, name))
}

func scan(row pgx.Row) (User, error) {
	var user User
	err := row.Scan(&user.ID, &user.Name, &user.Email, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return user, ErrNotFound
	}
	user.CreatedAt = user.CreatedAt.UTC()
	return user, err
}

type Service struct{ repo *Repository }

func NewService(repo *Repository) *Service { return &Service{repo: repo} }

func (s *Service) Profile(ctx context.Context, id string) (User, error) {
	return s.repo.Profile(ctx, id)
}

func (s *Service) Rename(ctx context.Context, id, input string) (User, error) {
	name, err := NormalizeName(input)
	if err != nil {
		return User{}, err
	}
	return s.repo.Rename(ctx, id, name)
}
