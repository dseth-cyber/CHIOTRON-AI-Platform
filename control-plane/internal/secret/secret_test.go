package secret

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func testKey(t *testing.T) string {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("read key: %v", err)
	}
	return base64.StdEncoding.EncodeToString(key)
}

func TestSealRoundTrips(t *testing.T) {
	sealer, err := NewSealer(testKey(t))
	if err != nil {
		t.Fatalf("NewSealer() returned error: %v", err)
	}

	const credential = "sk-test-0123456789abcdef"
	sealed, err := sealer.Seal(credential)
	if err != nil {
		t.Fatalf("Seal() returned error: %v", err)
	}
	// The stored bytes must not contain the credential: the whole point is that
	// reading a backup is not enough to use it.
	if strings.Contains(string(sealed), credential) {
		t.Fatal("the sealed value contains the plaintext")
	}

	opened, err := sealer.Open(sealed)
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	if opened != credential {
		t.Fatalf("Open() = %q, want %q", opened, credential)
	}
}

func TestSealUsesAFreshNonceEachTime(t *testing.T) {
	sealer, err := NewSealer(testKey(t))
	if err != nil {
		t.Fatalf("NewSealer() returned error: %v", err)
	}

	first, err := sealer.Seal("same-credential")
	if err != nil {
		t.Fatalf("Seal() returned error: %v", err)
	}
	second, err := sealer.Seal("same-credential")
	if err != nil {
		t.Fatalf("Seal() returned error: %v", err)
	}
	// Identical ciphertext for identical input would let somebody with the
	// database tell which providers share a key.
	if string(first) == string(second) {
		t.Fatal("sealing the same value twice produced identical ciphertext")
	}
}

func TestOpenRefusesAnotherKey(t *testing.T) {
	original, err := NewSealer(testKey(t))
	if err != nil {
		t.Fatalf("NewSealer() returned error: %v", err)
	}
	sealed, err := original.Seal("sk-test-0123456789abcdef")
	if err != nil {
		t.Fatalf("Seal() returned error: %v", err)
	}

	other, err := NewSealer(testKey(t))
	if err != nil {
		t.Fatalf("NewSealer() returned error: %v", err)
	}
	if _, err := other.Open(sealed); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("Open() with the wrong key error = %v, want ErrCorrupt", err)
	}
}

func TestOpenRefusesATamperedValue(t *testing.T) {
	sealer, err := NewSealer(testKey(t))
	if err != nil {
		t.Fatalf("NewSealer() returned error: %v", err)
	}
	sealed, err := sealer.Seal("sk-test-0123456789abcdef")
	if err != nil {
		t.Fatalf("Seal() returned error: %v", err)
	}

	// GCM authenticates the ciphertext, so a single flipped bit has to fail
	// rather than decrypt to something else.
	sealed[len(sealed)-1] ^= 0x01
	if _, err := sealer.Open(sealed); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("Open() on a tampered value error = %v, want ErrCorrupt", err)
	}
}

func TestNoKeyRefusesToSeal(t *testing.T) {
	// An empty key is legitimate for a deployment with only a local provider,
	// and it must refuse to store a credential rather than store it in the clear.
	sealer, err := NewSealer("")
	if err != nil {
		t.Fatalf("NewSealer(\"\") returned error: %v", err)
	}
	if sealer.Enabled() {
		t.Fatal("a sealer with no key reports itself enabled")
	}
	if _, err := sealer.Seal("sk-test"); !errors.Is(err, ErrNoKey) {
		t.Fatalf("Seal() with no key error = %v, want ErrNoKey", err)
	}
}

func TestNewSealerRejectsAKeyOfTheWrongLength(t *testing.T) {
	short := base64.StdEncoding.EncodeToString([]byte("too-short"))
	if _, err := NewSealer(short); err == nil {
		t.Fatal("NewSealer() accepted a key that is not 32 bytes")
	}
	if _, err := NewSealer("not base64 at all!!"); err == nil {
		t.Fatal("NewSealer() accepted a key that is not base64")
	}
}

func TestHintRevealsOnlyTheTail(t *testing.T) {
	if got := Hint("sk-test-0123456789abcdef"); got != "cdef" {
		t.Fatalf("Hint() = %q, want %q", got, "cdef")
	}
	// A short credential would be mostly given away by a four-character hint.
	if got := Hint("short"); got != "" {
		t.Fatalf("Hint() on a short credential = %q, want empty", got)
	}
}
