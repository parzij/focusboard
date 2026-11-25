package token

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestAccessValidation(t *testing.T) {
	const user = "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"
	key := []byte("jwt-signing-fixture-not-a-secret")
	access := NewAccess(key, "issuer", "audience", 15*time.Minute)
	now := time.Unix(1800000000, 0)
	access.now = func() time.Time { return now }
	raw, seconds, err := access.Issue(user)
	if err != nil || seconds != 900 {
		t.Fatalf("issue failed: %v", err)
	}
	if id, err := access.Verify(raw); err != nil || id != user {
		t.Fatalf("verify issued token: %v", err)
	}
	for _, test := range []struct {
		name   string
		method jwt.SigningMethod
		key    []byte
		change func(*jwt.RegisteredClaims)
	}{
		{"expired", nil, nil, func(c *jwt.RegisteredClaims) {
			c.IssuedAt = jwt.NewNumericDate(now.Add(-2 * time.Minute))
			c.ExpiresAt = jwt.NewNumericDate(now)
		}},
		{"wrong signature", nil, []byte("another-fixture"), nil},
		{"wrong algorithm", jwt.SigningMethodHS384, nil, nil},
		{"none algorithm", jwt.SigningMethodNone, nil, nil},
		{"wrong issuer", nil, nil, func(c *jwt.RegisteredClaims) { c.Issuer = "other" }},
		{"wrong audience", nil, nil, func(c *jwt.RegisteredClaims) { c.Audience = jwt.ClaimStrings{"other"} }},
		{"missing issuer", nil, nil, func(c *jwt.RegisteredClaims) { c.Issuer = "" }},
		{"missing audience", nil, nil, func(c *jwt.RegisteredClaims) { c.Audience = nil }},
		{"missing subject", nil, nil, func(c *jwt.RegisteredClaims) { c.Subject = "" }},
		{"missing expiration", nil, nil, func(c *jwt.RegisteredClaims) { c.ExpiresAt = nil }},
		{"missing issued at", nil, nil, func(c *jwt.RegisteredClaims) { c.IssuedAt = nil }},
		{"missing jti", nil, nil, func(c *jwt.RegisteredClaims) { c.ID = "" }},
		{"invalid subject", nil, nil, func(c *jwt.RegisteredClaims) { c.Subject = "not-a-uuid" }},
		{"future issued at", nil, nil, func(c *jwt.RegisteredClaims) { c.IssuedAt = jwt.NewNumericDate(now.Add(time.Minute)) }},
		{"long lifetime", nil, nil, func(c *jwt.RegisteredClaims) { c.ExpiresAt = jwt.NewNumericDate(now.Add(time.Hour)) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			claims := jwt.RegisteredClaims{Subject: user, ID: user, Issuer: "issuer", Audience: jwt.ClaimStrings{"audience"},
				IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute))}
			if test.change != nil {
				test.change(&claims)
			}
			method := test.method
			if method == nil {
				method = jwt.SigningMethodHS256
			}
			var signingKey any = key
			if test.key != nil {
				signingKey = test.key
			}
			if method == jwt.SigningMethodNone {
				signingKey = jwt.UnsafeAllowNoneSignatureType
			}
			raw, err := jwt.NewWithClaims(method, claims).SignedString(signingKey)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := access.Verify(raw); err == nil {
				t.Fatal("invalid JWT accepted")
			}
		})
	}
}

func TestRefreshHash(t *testing.T) {
	value, hash, err := NewRefresh()
	if got, ok := RefreshHash(value); err != nil || !ok || got != hash || len(hash) != 64 {
		t.Fatal("refresh digest mismatch")
	}
	for _, invalid := range []string{"", value + "=", strings.Repeat("!", 43)} {
		if _, ok := RefreshHash(invalid); ok {
			t.Fatal("invalid refresh token accepted")
		}
	}
}
