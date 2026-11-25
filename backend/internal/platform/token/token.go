package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var errInvalid = errors.New("invalid access token")

type Access struct {
	key              []byte
	issuer, audience string
	ttl              time.Duration
	now              func() time.Time
}

func NewAccess(key []byte, issuer, audience string, ttl time.Duration) *Access {
	return &Access{key: append([]byte(nil), key...), issuer: issuer, audience: audience, ttl: ttl, now: time.Now}
}

func (a *Access) Issue(userID string) (string, int, error) {
	now := a.now().UTC().Truncate(time.Second)
	id, err := uuid.NewRandom()
	if err != nil {
		return "", 0, err
	}
	claims := jwt.RegisteredClaims{
		Subject: userID, Issuer: a.issuer, Audience: jwt.ClaimStrings{a.audience},
		IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(a.ttl)), ID: id.String(),
	}
	value, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(a.key)
	return value, int(a.ttl / time.Second), err
}

func (a *Access) Verify(value string) (string, error) {
	if len(value) > 4096 {
		return "", errInvalid
	}
	var claims jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(value, &claims, func(_ *jwt.Token) (any, error) {
		return a.key, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired(),
		jwt.WithIssuer(a.issuer), jwt.WithAudience(a.audience), jwt.WithIssuedAt(), jwt.WithTimeFunc(a.now), jwt.WithStrictDecoding())
	if err != nil || claims.IssuedAt == nil || !claims.ExpiresAt.After(claims.IssuedAt.Time) || claims.ExpiresAt.Sub(claims.IssuedAt.Time) > a.ttl ||
		!validID(claims.Subject) || !validID(claims.ID) {
		return "", errInvalid
	}
	return claims.Subject, nil
}

func validID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}

func NewRefresh() (value, hash string, err error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", "", err
	}
	value = base64.RawURLEncoding.EncodeToString(raw[:])
	sum := sha256.Sum256(raw[:])
	return value, hex.EncodeToString(sum[:]), nil
}

func RefreshHash(value string) (string, bool) {
	if len(value) != base64.RawURLEncoding.EncodedLen(32) {
		return "", false
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(value)
	if err != nil || len(raw) != 32 || base64.RawURLEncoding.EncodeToString(raw) != value {
		return "", false
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), true
}
