package auth

import (
	"errors"
	"testing"
	"time"
)

func TestJWTValidationAndActiveCompanyGuard(t *testing.T) {
	secret := "super-secure-enterprise-jwt-signing-secret"
	validator := NewJWTValidator(secret, "https://id.enterprise.internal")

	// 1. Valid Token
	claims := JWTClaims{
		Sub:               "user-999",
		Name:              "Alice Engineer",
		CompanyID:         "acme-corp",
		Department:        "engineering",
		MaxClassification: "restricted",
		Scopes:            []string{ScopeChatCompletion, ScopeKnowledgeRead},
		Exp:               time.Now().Add(1 * time.Hour).Unix(),
	}

	token, err := validator.Sign(claims)
	if err != nil {
		t.Fatalf("Sign error: %v", err)
	}

	identity, err := validator.Validate(token)
	if err != nil {
		t.Fatalf("Validate error: %v", err)
	}

	if identity.KeyID != "user-999" || identity.CompanyID != "acme-corp" {
		t.Errorf("Unexpected identity extracted: %+v", identity)
	}
	if !identity.HasScope(ScopeChatCompletion) {
		t.Errorf("Expected ScopeChatCompletion to be granted")
	}

	// 2. Active Company Guard (Successful Match)
	if err := ActiveCompanyGuard(identity, "acme-corp", "active"); err != nil {
		t.Errorf("ActiveCompanyGuard failed for matching active company: %v", err)
	}

	// 3. Active Company Guard (Inactive Company)
	if err := ActiveCompanyGuard(identity, "acme-corp", "suspended"); !errors.Is(err, ErrInactiveCompany) {
		t.Errorf("Expected ErrInactiveCompany on suspended company, got %v", err)
	}

	// 4. Active Company Guard (Company Mismatch)
	if err := ActiveCompanyGuard(identity, "other-corp", "active"); !errors.Is(err, ErrCompanyMismatch) {
		t.Errorf("Expected ErrCompanyMismatch on company mismatch, got %v", err)
	}

	// 5. Expired Token
	expiredClaims := claims
	expiredClaims.Exp = time.Now().Add(-1 * time.Hour).Unix()
	expiredToken, _ := validator.Sign(expiredClaims)

	if _, err := validator.Validate(expiredToken); !errors.Is(err, ErrTokenExpired) {
		t.Errorf("Expected ErrTokenExpired on expired token, got %v", err)
	}

	// 6. Invalid Signature Token
	otherValidator := NewJWTValidator("wrong-secret", "https://id.enterprise.internal")
	tamperedToken, _ := otherValidator.Sign(claims)

	if _, err := validator.Validate(tamperedToken); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("Expected ErrInvalidToken on signature mismatch, got %v", err)
	}
}
