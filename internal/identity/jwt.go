package identity

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// JWTClaims are the claims extracted from a verified JWT.
type JWTClaims struct {
	AgentID     string `json:"agent_id"`
	TeamID      string `json:"team_id"`
	ProjectID   string `json:"project_id"`
	Environment string `json:"environment"`
	Exp         int64  `json:"exp"`
	Iat         int64  `json:"iat"`
}

// VerifyJWT verifies an HMAC-SHA256 JWT and returns the claims.
// Returns an error if the token is malformed, the signature is invalid,
// or the token is expired.
func VerifyJWT(token string, secret []byte) (*JWTClaims, error) {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("malformed JWT: expected 3 parts, got %d", len(parts))
	}

	// Verify header is HS256.
	headerJSON, err := base64URLDecode(parts[0])
	if err != nil {
		return nil, fmt.Errorf("decode header: %w", err)
	}
	var header struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, fmt.Errorf("parse header: %w", err)
	}
	if header.Alg != "HS256" {
		return nil, fmt.Errorf("unsupported algorithm: %s", header.Alg)
	}

	// Verify signature.
	signingInput := parts[0] + "." + parts[1]
	expectedSig := hmacSHA256([]byte(signingInput), secret)
	gotSig, err := base64URLDecode(parts[2])
	if err != nil {
		return nil, fmt.Errorf("decode signature: %w", err)
	}
	if !hmac.Equal(expectedSig, gotSig) {
		return nil, fmt.Errorf("invalid signature")
	}

	// Decode claims.
	claimsJSON, err := base64URLDecode(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode claims: %w", err)
	}
	var claims JWTClaims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, fmt.Errorf("parse claims: %w", err)
	}

	// Check expiry.
	if claims.Exp > 0 && time.Now().Unix() > claims.Exp {
		return nil, fmt.Errorf("token expired")
	}

	if claims.AgentID == "" {
		return nil, fmt.Errorf("missing agent_id claim")
	}

	return &claims, nil
}

// SignJWT creates an HMAC-SHA256 JWT from the given claims.
func SignJWT(claims JWTClaims, secret []byte) string {
	header := base64URLEncode([]byte(`{"alg":"HS256","typ":"JWT"}`))
	claimsJSON, _ := json.Marshal(claims)
	payload := base64URLEncode(claimsJSON)
	signingInput := header + "." + payload
	sig := base64URLEncode(hmacSHA256([]byte(signingInput), secret))
	return signingInput + "." + sig
}

func hmacSHA256(data, key []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func base64URLEncode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func base64URLDecode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}
