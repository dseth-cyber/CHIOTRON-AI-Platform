// Package secret seals credentials the platform stores on behalf of an
// operator.
//
// Provider API keys are not the platform's own secrets: they are somebody's
// paid account, they are worth money to an attacker, and a database backup is
// not a place to keep them in the clear. They are sealed with AES-GCM under a
// key that comes from the environment, so reading the database is not enough to
// use them (ARCHITECTURE-v1 section 5).
package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

// ErrNoKey reports that no encryption key is configured. Callers treat it as a
// refusal to store a credential rather than as a reason to store it in the
// clear.
var ErrNoKey = errors.New("no credential encryption key is configured")

// ErrCorrupt reports a sealed value that will not open. It is distinct from
// ErrNoKey because the operator response differs: a rotated key needs the old
// key back, a corrupt row needs the credential entering again.
var ErrCorrupt = errors.New("sealed credential could not be opened")

// Sealer encrypts and decrypts stored credentials.
//
// The zero value refuses to seal anything, which is what makes a deployment
// without CONFIG_ENCRYPTION_KEY fail closed: providers that need no credential
// still work, and one that needs a credential cannot be created.
type Sealer struct {
	aead cipher.AEAD
}

// NewSealer builds a sealer from a base64 32-byte key.
//
// An empty key is not an error: a development deployment running only a local
// Ollama has no credential to protect, and demanding a key it does not need
// would push an operator into inventing one and committing it.
func NewSealer(encoded string) (*Sealer, error) {
	if encoded == "" {
		return &Sealer{}, nil
	}
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("credential encryption key is not valid base64: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("credential encryption key must decode to 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("build cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("build AEAD: %w", err)
	}
	return &Sealer{aead: aead}, nil
}

// Enabled reports whether credentials can be stored at all.
func (s *Sealer) Enabled() bool { return s != nil && s.aead != nil }

// Seal encrypts a credential. The nonce is prepended to the ciphertext, so one
// stored value is self-contained and no second column has to stay in step
// with it.
func (s *Sealer) Seal(plaintext string) ([]byte, error) {
	if !s.Enabled() {
		return nil, ErrNoKey
	}
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("read nonce: %w", err)
	}
	return s.aead.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

// Open decrypts a sealed credential.
func (s *Sealer) Open(sealed []byte) (string, error) {
	if !s.Enabled() {
		return "", ErrNoKey
	}
	if len(sealed) < s.aead.NonceSize() {
		return "", ErrCorrupt
	}
	nonce, ciphertext := sealed[:s.aead.NonceSize()], sealed[s.aead.NonceSize():]
	plaintext, err := s.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		// The underlying error distinguishes a bad key from a mangled byte, but
		// reporting which would tell an attacker whether the key was close.
		return "", ErrCorrupt
	}
	return string(plaintext), nil
}

// Hint is the tail of a credential, for telling two keys apart in a UI.
//
// Four characters of a random key identify it to somebody who already has it
// and are useless to somebody who does not. A short credential yields no hint
// at all rather than most of itself.
func Hint(credential string) string {
	if len(credential) < 12 {
		return ""
	}
	return credential[len(credential)-4:]
}
