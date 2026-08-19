// Package auth: JWT Token Validation and Active-Company Guard.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidToken    = errors.New("invalid or malformed jwt token")
	ErrTokenExpired    = errors.New("jwt token is expired")
	ErrInactiveCompany = errors.New("company is inactive or suspended")
	ErrCompanyMismatch = errors.New("caller company does not match active company context")
)

// JWTClaims represents identity and authorization claims issued by the Enterprise Identity Provider.
type JWTClaims struct {
	Sub               string   `json:"sub"`
	Name              string   `json:"name"`
	CompanyID         string   `json:"company_id"`
	Department        string   `json:"department"`
	MaxClassification string   `json:"max_classification"`
	Scopes            []string `json:"scopes"`
	Roles             []string `json:"roles,omitempty"`
	Exp               int64    `json:"exp"`
	Iss               string   `json:"iss,omitempty"`
	Aud               string   `json:"aud,omitempty"`
}

// JWTValidator defines the interface for verifying Enterprise Identity tokens.
type JWTValidator struct {
	secret []byte
	issuer string
}

// NewJWTValidator creates a new validator with the given signing secret.
func NewJWTValidator(secret string, issuer string) *JWTValidator {
	return &JWTValidator{
		secret: []byte(secret),
		issuer: issuer,
	}
}

// Sign generates an HMAC-SHA256 JWT token for testing and inter-service authentication.
func (v *JWTValidator) Sign(claims JWTClaims) (string, error) {
	header := map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signingInput := fmt.Sprintf("%s.%s", headerB64, claimsB64)

	mac := hmac.New(sha256.New, v.secret)
	mac.Write([]byte(signingInput))
	sigB64 := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return fmt.Sprintf("%s.%s", signingInput, sigB64), nil
}

// Validate parses, verifies the signature, and returns the authenticated Identity.
func (v *JWTValidator) Validate(tokenString string) (Identity, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return Identity{}, ErrInvalidToken
	}

	signingInput := fmt.Sprintf("%s.%s", parts[0], parts[1])
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Identity{}, ErrInvalidToken
	}

	mac := hmac.New(sha256.New, v.secret)
	mac.Write([]byte(signingInput))
	expectedSig := mac.Sum(nil)

	if !hmac.Equal(sig, expectedSig) {
		return Identity{}, ErrInvalidToken
	}

	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Identity{}, ErrInvalidToken
	}

	var claims JWTClaims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return Identity{}, ErrInvalidToken
	}

	if claims.Exp > 0 && time.Now().UTC().Unix() > claims.Exp {
		return Identity{}, ErrTokenExpired
	}

	maxClass := claims.MaxClassification
	if maxClass == "" {
		maxClass = "internal"
	}

	return Identity{
		KeyID:              claims.Sub,
		Name:               claims.Name,
		Scopes:             claims.Scopes,
		CompanyID:          claims.CompanyID,
		Department:         claims.Department,
		MaxClassification:  maxClass,
		RateLimitPerMinute: 120,
	}, nil
}

// ActiveCompanyGuard verifies that the caller's company matches the target tenant and is in active standing.
func ActiveCompanyGuard(id Identity, targetCompanyID string, companyStatus string) error {
	if companyStatus != "" && companyStatus != "active" {
		return fmt.Errorf("%w: company status is %q", ErrInactiveCompany, companyStatus)
	}
	if targetCompanyID != "" && id.CompanyID != "" && id.CompanyID != targetCompanyID {
		return fmt.Errorf("%w: token company %q does not match target company %q",
			ErrCompanyMismatch, id.CompanyID, targetCompanyID)
	}
	return nil
}
