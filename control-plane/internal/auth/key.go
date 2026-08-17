// Package auth implements platform-owned API key credentials.
//
// API keys are hashed, scoped, rate-limited, expirable and auditable, and the
// raw value is shown once only (ARCHITECTURE-v1 section 5). They are separate
// from the JWTs issued by the existing Identity Service, which arrive with the
// identity integration.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// Prefix marks a Control Plane key so a leaked value is recognisable in logs
// and secret scanners.
const keyPrefix = "ceap"

const (
	// The prefix is hex so it can never contain the `_` separator. The secret
	// is base64url for compactness and may contain `_`, which is why parsing
	// splits into exactly three fields rather than on every separator.
	prefixBytes = 8  // 16 hex characters
	secretBytes = 32 // 256 bits of randomness
)

// Scopes gate what a key may do. The backend authorizes every action; UI
// filtering is convenience only (ARCHITECTURE-v1 section 5).
const (
	ScopeModelsRead     = "models:read"
	ScopeAssistantsRead = "assistants:read"
	// ScopeChatCompletion also covers reading and deleting the caller's own
	// conversations: a transcript is part of using chat, not a separate
	// capability, and it is only ever visible to the credential that created it.
	ScopeChatCompletion  = "chat:completions"
	ScopeAdminKeys       = "admin:keys"
	ScopeAdminAssistants = "admin:assistants"
)

// KnownScopes lists every scope the Control Plane understands today.
var KnownScopes = []string{
	ScopeModelsRead, ScopeAssistantsRead, ScopeChatCompletion,
	ScopeAdminKeys, ScopeAdminAssistants,
}

var (
	ErrMalformedKey = errors.New("malformed api key")
	ErrUnknownScope = errors.New("unknown scope")
)

// Generated is a freshly minted key. Secret is the only time the raw value
// exists; it is never stored and never logged.
type Generated struct {
	Prefix     string
	SecretHash string
	Secret     string
}

// Generate mints a key in the form `ceap_<prefix>_<secret>`.
func Generate() (Generated, error) {
	prefix, err := randomHex(prefixBytes)
	if err != nil {
		return Generated{}, err
	}
	secret, err := randomToken(secretBytes)
	if err != nil {
		return Generated{}, err
	}
	return Generated{
		Prefix:     prefix,
		SecretHash: HashSecret(secret),
		Secret:     fmt.Sprintf("%s_%s_%s", keyPrefix, prefix, secret),
	}, nil
}

// Parse splits a presented key into its lookup prefix and secret half.
//
// The split is limited to three fields because the secret is base64url encoded
// and may legitimately contain the `_` separator; the prefix is hex and cannot.
func Parse(presented string) (prefix, secret string, err error) {
	parts := strings.SplitN(strings.TrimSpace(presented), "_", 3)
	if len(parts) != 3 || parts[0] != keyPrefix || parts[1] == "" || parts[2] == "" {
		return "", "", ErrMalformedKey
	}
	return parts[1], parts[2], nil
}

func HashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// SecretMatches compares in constant time so a timing signal cannot be used to
// recover a stored hash.
func SecretMatches(secret, storedHash string) bool {
	return subtle.ConstantTimeCompare([]byte(HashSecret(secret)), []byte(storedHash)) == 1
}

// NormalizeScopes rejects unknown scopes and removes duplicates, so a typo
// fails at key creation rather than silently granting nothing.
func NormalizeScopes(scopes []string) ([]string, error) {
	known := make(map[string]bool, len(KnownScopes))
	for _, scope := range KnownScopes {
		known[scope] = true
	}

	seen := make(map[string]bool, len(scopes))
	normalized := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			continue
		}
		if !known[scope] {
			return nil, fmt.Errorf("%w: %q (known scopes: %s)", ErrUnknownScope, scope, strings.Join(KnownScopes, ", "))
		}
		if seen[scope] {
			continue
		}
		seen[scope] = true
		normalized = append(normalized, scope)
	}
	if len(normalized) == 0 {
		return nil, fmt.Errorf("at least one scope is required (known scopes: %s)", strings.Join(KnownScopes, ", "))
	}
	return normalized, nil
}

func randomToken(size int) (string, error) {
	buffer, err := randomBytes(size)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func randomHex(size int) (string, error) {
	buffer, err := randomBytes(size)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

func randomBytes(size int) ([]byte, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return nil, fmt.Errorf("generate random token: %w", err)
	}
	return buffer, nil
}
