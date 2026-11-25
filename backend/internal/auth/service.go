package auth

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/parzij/focusboard/backend/internal/platform/password"
	"github.com/parzij/focusboard/backend/internal/platform/token"
	"github.com/parzij/focusboard/backend/internal/users"
)

var (
	ErrCredentials = errors.New("invalid credentials")
	ErrEmailExists = errors.New("email already exists")
	ErrSession     = errors.New("invalid refresh session")
)

type ValidationError map[string]string

func (ValidationError) Error() string { return "invalid fields" }

type Result struct {
	AccessToken  string
	ExpiresIn    int
	User         users.User
	Refresh      string
	RefreshUntil time.Time
}

type Service struct {
	repo       *Repository
	passwords  *password.Hasher
	access     *token.Access
	refreshTTL time.Duration
}

func NewService(repo *Repository, passwords *password.Hasher, access *token.Access, refreshTTL time.Duration) *Service {
	return &Service{repo: repo, passwords: passwords, access: access, refreshTTL: refreshTTL}
}

const passwordError = "Password must contain 8–128 characters and at most 512 bytes."

func (s *Service) Register(ctx context.Context, name, email, secret string) (Result, error) {
	email = normalizeEmail(email)
	fields := make(ValidationError)
	name, err := users.NormalizeName(name)
	if err != nil {
		fields["name"] = users.NameError
	}
	if email == "" {
		fields["email"] = "Enter a valid email address."
	}
	if !password.ValidNew(secret) {
		fields["password"] = passwordError
	}
	if len(fields) > 0 {
		return Result{}, fields
	}
	hash, err := s.passwords.Hash(ctx, secret)
	if err != nil {
		return Result{}, err
	}
	value, sessionHash, err := token.NewRefresh()
	if err != nil {
		return Result{}, err
	}
	expiry := time.Now().UTC().Add(s.refreshTTL)
	user, err := s.repo.Register(ctx, name, email, hash, sessionHash, expiry)
	if err != nil {
		return Result{}, err
	}
	return s.result(user, value, expiry)
}

func (s *Service) Login(ctx context.Context, email, secret string) (Result, error) {
	email = normalizeEmail(email)
	if !password.ValidExisting(secret) || email == "" {
		return Result{}, ErrCredentials
	}
	id, hash, err := s.repo.CredentialsByEmail(ctx, email)
	if errors.Is(err, users.ErrNotFound) {
		if err := s.passwords.Dummy(ctx, secret); err != nil {
			return Result{}, err
		}
		return Result{}, ErrCredentials
	}
	if err != nil {
		return Result{}, err
	}
	valid, err := s.passwords.Verify(ctx, secret, hash)
	if err != nil {
		return Result{}, err
	}
	if !valid {
		return Result{}, ErrCredentials
	}
	value, sessionHash, err := token.NewRefresh()
	if err != nil {
		return Result{}, err
	}
	expiry := time.Now().UTC().Add(s.refreshTTL)
	user, err := s.repo.Login(ctx, id, hash, sessionHash, expiry)
	if err != nil {
		return Result{}, err
	}
	return s.result(user, value, expiry)
}

func (s *Service) Refresh(ctx context.Context, cookie string) (Result, error) {
	oldHash, valid := token.RefreshHash(cookie)
	if !valid {
		return Result{}, ErrSession
	}
	value, newHash, err := token.NewRefresh()
	if err != nil {
		return Result{}, err
	}
	user, expiry, err := s.repo.Rotate(ctx, oldHash, newHash)
	if err != nil {
		return Result{}, err
	}
	return s.result(user, value, expiry)
}

func (s *Service) Logout(ctx context.Context, cookie string) error {
	if hash, valid := token.RefreshHash(cookie); valid {
		return s.repo.Logout(ctx, hash)
	}
	return nil
}

func (s *Service) ChangePassword(ctx context.Context, id, current, replacement string) error {
	if !password.ValidNew(replacement) {
		return ValidationError{"newPassword": passwordError}
	}
	if !password.ValidExisting(current) {
		return ErrCredentials
	}
	hash, err := s.repo.PasswordHash(ctx, id)
	if err != nil {
		return err
	}
	valid, err := s.passwords.Verify(ctx, current, hash)
	if err != nil {
		return err
	}
	if !valid {
		return ErrCredentials
	}
	newHash, err := s.passwords.Hash(ctx, replacement)
	if err != nil {
		return err
	}
	return s.repo.ChangePassword(ctx, id, hash, newHash)
}

func (s *Service) result(user users.User, refresh string, expiry time.Time) (Result, error) {
	access, seconds, err := s.access.Issue(user.ID)
	user.CreatedAt = user.CreatedAt.UTC()
	return Result{AccessToken: access, ExpiresIn: seconds, User: user, Refresh: refresh, RefreshUntil: expiry}, err
}

func normalizeEmail(input string) string {
	email := strings.ToLower(strings.TrimSpace(input))
	if len(email) > 254 {
		return ""
	}
	for _, c := range email {
		if c < 33 || c > 126 {
			return ""
		}
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Name != "" || addr.Address != email {
		return ""
	}
	return email
}
