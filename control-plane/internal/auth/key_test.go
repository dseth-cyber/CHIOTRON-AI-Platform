package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestGenerateProducesParseableKey(t *testing.T) {
	generated, err := Generate()
	if err != nil {
		t.Fatalf("Generate() returned error: %v", err)
	}

	if !strings.HasPrefix(generated.Secret, "ceap_") {
		t.Errorf("secret = %q, want the ceap_ marker so a leaked value is recognisable", generated.Secret)
	}

	prefix, secret, err := Parse(generated.Secret)
	if err != nil {
		t.Fatalf("Parse() returned error for a generated key: %v", err)
	}
	if prefix != generated.Prefix {
		t.Errorf("parsed prefix = %q, want %q", prefix, generated.Prefix)
	}
	if !SecretMatches(secret, generated.SecretHash) {
		t.Error("the generated secret does not match its stored hash")
	}
	// The raw value must never be recoverable from what is stored.
	if strings.Contains(generated.SecretHash, secret) {
		t.Error("the stored hash contains the secret")
	}
}

func TestGenerateIsUnique(t *testing.T) {
	seen := make(map[string]bool)
	for range 100 {
		generated, err := Generate()
		if err != nil {
			t.Fatalf("Generate() returned error: %v", err)
		}
		if seen[generated.Prefix] {
			t.Fatalf("prefix %q was generated twice", generated.Prefix)
		}
		seen[generated.Prefix] = true
	}
}

func TestParseRejectsMalformedKeys(t *testing.T) {
	cases := map[string]string{
		"empty":          "",
		"wrong marker":   "sk_abc_def",
		"missing secret": "ceap_abc_",
		"missing prefix": "ceap__def",
		"too few parts":  "ceap_abc",
	}
	for name, presented := range cases {
		t.Run(name, func(t *testing.T) {
			if _, _, err := Parse(presented); !errors.Is(err, ErrMalformedKey) {
				t.Errorf("Parse(%q) error = %v, want ErrMalformedKey", presented, err)
			}
		})
	}
}

// The secret is base64url encoded, so it may legitimately contain the `_`
// separator. Splitting on every `_` would reject roughly half of all keys.
func TestParseAcceptsUnderscoreInSecret(t *testing.T) {
	prefix, secret, err := Parse("ceap_0123456789abcdef_aa_bb-cc")
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}
	if prefix != "0123456789abcdef" {
		t.Errorf("prefix = %q, want the hex lookup handle", prefix)
	}
	if secret != "aa_bb-cc" {
		t.Errorf("secret = %q, want the remainder intact", secret)
	}
}

// The prefix is the database lookup handle, so it must never contain the
// separator itself.
func TestGeneratedPrefixHasNoSeparator(t *testing.T) {
	for range 50 {
		generated, err := Generate()
		if err != nil {
			t.Fatalf("Generate() returned error: %v", err)
		}
		if strings.Contains(generated.Prefix, "_") {
			t.Fatalf("generated prefix %q contains the separator", generated.Prefix)
		}
	}
}

func TestSecretMatchesRejectsWrongSecret(t *testing.T) {
	hash := HashSecret("correct")

	if !SecretMatches("correct", hash) {
		t.Error("SecretMatches rejected the correct secret")
	}
	if SecretMatches("wrong", hash) {
		t.Error("SecretMatches accepted the wrong secret")
	}
}

// A scope typo must fail at key creation rather than silently granting nothing.
func TestNormalizeScopesRejectsUnknownScope(t *testing.T) {
	_, err := NormalizeScopes([]string{ScopeModelsRead, "chat:complete"})
	if !errors.Is(err, ErrUnknownScope) {
		t.Fatalf("error = %v, want ErrUnknownScope", err)
	}
	if !strings.Contains(err.Error(), ScopeChatCompletion) {
		t.Errorf("error %q does not list the known scopes", err)
	}
}

func TestNormalizeScopesDeduplicatesAndTrims(t *testing.T) {
	scopes, err := NormalizeScopes([]string{" models:read ", "models:read", "", ScopeAdminKeys})
	if err != nil {
		t.Fatalf("NormalizeScopes() returned error: %v", err)
	}
	want := []string{ScopeModelsRead, ScopeAdminKeys}
	if len(scopes) != len(want) {
		t.Fatalf("scopes = %v, want %v", scopes, want)
	}
	for i := range want {
		if scopes[i] != want[i] {
			t.Errorf("scopes[%d] = %q, want %q", i, scopes[i], want[i])
		}
	}
}

func TestNormalizeScopesRequiresAtLeastOne(t *testing.T) {
	if _, err := NormalizeScopes([]string{"", "  "}); err == nil {
		t.Fatal("NormalizeScopes() accepted an empty scope list, want error")
	}
}

func TestHasScope(t *testing.T) {
	identity := Identity{Scopes: []string{ScopeModelsRead}}

	if !identity.HasScope(ScopeModelsRead) {
		t.Error("HasScope rejected a granted scope")
	}
	if identity.HasScope(ScopeAdminKeys) {
		t.Error("HasScope accepted a scope that was not granted")
	}
}
