package password

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

const (
	memory      = 64 * 1024
	iterations  = 3
	parallelism = 1
	saltSize    = 16
	keySize     = 32
	parameters  = "m=65536,t=3,p=1"
	MaxBytes    = 512
)

var ErrBusy = errors.New("password worker limit reached")

type Hasher struct {
	slots chan struct{}
	dummy string
}

func New(workers int) (*Hasher, error) {
	h := &Hasher{slots: make(chan struct{}, workers)}
	var err error
	h.dummy, err = h.Hash(context.Background(), rand.Text())
	return h, err
}

func ValidNew(value string) bool {
	return ValidExisting(value) && utf8.RuneCountInString(value) >= 8
}

func ValidExisting(value string) bool {
	return len(value) > 0 && len(value) <= MaxBytes && utf8.ValidString(value) && utf8.RuneCountInString(value) <= 128
}

func (h *Hasher) Hash(ctx context.Context, value string) (string, error) {
	if !ValidExisting(value) {
		return "", errors.New("password exceeds supported limits")
	}
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := h.derive(ctx, value, salt)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("$argon2id$v=%d$%s$%s$%s", argon2.Version, parameters,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

func (h *Hasher) Verify(ctx context.Context, value, encoded string) (bool, error) {
	if !ValidExisting(value) {
		return false, nil
	}
	salt, expected, valid := parse(encoded)
	if !valid {
		// Run the same work for an unusable stored credential.
		salt, expected, _ = parse(h.dummy)
	}
	key, err := h.derive(ctx, value, salt)
	matches := subtle.ConstantTimeCompare(key, expected) == 1
	return err == nil && valid && matches, err
}

func (h *Hasher) Dummy(ctx context.Context, value string) error {
	_, err := h.Verify(ctx, value, h.dummy)
	return err
}

func parse(encoded string) (salt, hash []byte, valid bool) {
	if len(encoded) > 128 {
		return nil, nil, false
	}
	parts := strings.Split(encoded, "$")
	// Accept only the current cost profile. Add another explicitly bounded profile when rotating parameters.
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v=19" ||
		parts[3] != parameters {
		return nil, nil, false
	}
	encoding := base64.RawStdEncoding.Strict()
	salt, saltErr := encoding.DecodeString(parts[4])
	hash, hashErr := encoding.DecodeString(parts[5])
	valid = saltErr == nil && hashErr == nil && len(salt) == saltSize && len(hash) == keySize &&
		encoding.EncodeToString(salt) == parts[4] && encoding.EncodeToString(hash) == parts[5]
	return salt, hash, valid
}

func (h *Hasher) derive(ctx context.Context, value string, salt []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case h.slots <- struct{}{}:
		defer func() { <-h.slots }()
	default:
		return nil, ErrBusy
	}
	key := argon2.IDKey([]byte(value), salt, iterations, memory, parallelism, keySize)
	return key, ctx.Err()
}
