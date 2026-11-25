package password

import (
	"errors"
	"strings"
	"testing"
)

const sample = "  password 🔑 sample  "

func TestHashAndVerify(t *testing.T) {
	h, err := New(1)
	if err != nil {
		t.Fatal(err)
	}
	first, err := h.Hash(t.Context(), sample)
	if err != nil {
		t.Fatal(err)
	}
	second, err := h.Hash(t.Context(), sample)
	if err != nil {
		t.Fatal(err)
	}
	salt, _, valid := parse(first)
	otherSalt, _, otherValid := parse(second)
	if !valid || !otherValid || string(salt) == string(otherSalt) {
		t.Fatal("hashes did not use distinct salts")
	}
	for _, test := range []struct {
		name, value, encoded string
		want                 bool
	}{
		{"correct", sample, first, true},
		{"wrong", "a different password", first, false},
		{"untrimmed", strings.TrimSpace(sample), first, false},
		{"corrupt", sample, "$argon2id$broken", false},
		{"unsafe memory", sample, strings.Replace(first, "65536", "4294967295", 1), false},
		{"unsafe CPU", sample, strings.Replace(first, "t=3", "t=2000000", 1), false},
		{"unsafe parallelism", sample, strings.Replace(first, "p=1", "p=255", 1), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			ok, err := h.Verify(t.Context(), test.value, test.encoded)
			if err != nil || ok != test.want {
				t.Fatalf("verify returned %v, %v", ok, err)
			}
		})
	}
	h.slots <- struct{}{}
	defer func() { <-h.slots }()
	if _, err := h.Hash(t.Context(), sample); !errors.Is(err, ErrBusy) {
		t.Fatal("worker saturation did not reject work")
	}
}

func TestPasswordLengths(t *testing.T) {
	for _, test := range []struct {
		value             string
		newOK, existingOK bool
	}{
		{"", false, false}, {strings.Repeat("x", 7), false, true}, {strings.Repeat("x", 8), true, true},
		{strings.Repeat("🔑", 128), true, true}, {strings.Repeat("🔑", 129), false, false},
		{strings.Repeat("x", 129), false, false}, {"\xffnot valid UTF-8", false, false},
	} {
		if ValidNew(test.value) != test.newOK || ValidExisting(test.value) != test.existingOK {
			t.Fatalf("unexpected validation for %d bytes", len(test.value))
		}
	}
}

func BenchmarkHash(b *testing.B) {
	h, err := New(1)
	if err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		if _, err := h.Hash(b.Context(), sample); err != nil {
			b.Fatal(err)
		}
	}
}
