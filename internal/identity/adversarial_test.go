package identity

import (
	"testing"
	"time"
)

// TestAdversarialExpiredJWT verifies expired tokens are always rejected.
func TestAdversarialExpiredJWT(t *testing.T) {
	secret := []byte("secret")
	claims := JWTClaims{AgentID: "a1", Exp: time.Now().Add(-time.Second).Unix()}
	token := SignJWT(claims, secret)

	_, err := VerifyJWT(token, secret)
	if err == nil {
		t.Fatal("expired token must be rejected")
	}
}

// TestAdversarialAlgNone verifies "alg":"none" tokens are rejected.
func TestAdversarialAlgNone(t *testing.T) {
	// Craft a token with alg=none (classic JWT bypass attack).
	header := base64URLEncode([]byte(`{"alg":"none","typ":"JWT"}`))
	payload := base64URLEncode([]byte(`{"agent_id":"admin","exp":9999999999}`))
	token := header + "." + payload + "."

	_, err := VerifyJWT(token, []byte("any-secret"))
	if err == nil {
		t.Fatal("alg=none must be rejected")
	}
}

// TestAdversarialAlgSwitch verifies algorithm switching is rejected.
func TestAdversarialAlgSwitch(t *testing.T) {
	// Try RS256 header with HMAC signature.
	header := base64URLEncode([]byte(`{"alg":"RS256","typ":"JWT"}`))
	payload := base64URLEncode([]byte(`{"agent_id":"a1","exp":9999999999}`))
	token := header + "." + payload + ".fakesignature"

	_, err := VerifyJWT(token, []byte("secret"))
	if err == nil {
		t.Fatal("algorithm switch must be rejected")
	}
}

// TestAdversarialEmptySecret verifies empty secret doesn't accidentally validate.
func TestAdversarialEmptySecret(t *testing.T) {
	claims := JWTClaims{AgentID: "a1", Exp: time.Now().Add(time.Hour).Unix()}
	token := SignJWT(claims, []byte("real-secret"))

	_, err := VerifyJWT(token, []byte(""))
	if err == nil {
		t.Fatal("empty secret should not validate a token signed with real secret")
	}
}

// TestAdversarialOversizedToken verifies huge tokens don't cause OOM.
func TestAdversarialOversizedToken(t *testing.T) {
	huge := string(make([]byte, 1<<20)) // 1MB
	_, err := VerifyJWT(huge, []byte("s"))
	if err == nil {
		t.Fatal("oversized token should be rejected")
	}
}

// TestAdversarialRegistryLookupEmpty verifies empty/null key lookups.
func TestAdversarialRegistryLookupEmpty(t *testing.T) {
	reg := &Registry{
		byKey: map[string]*Agent{
			"valid-key": {AgentID: "a1", APIKey: "valid-key", Status: "active"},
		},
		byID: map[string]*Agent{},
	}

	if reg.LookupByKey("") != nil {
		t.Fatal("empty key should return nil")
	}
	if reg.LookupByKey("\x00") != nil {
		t.Fatal("null byte key should return nil")
	}
	if reg.LookupByKey(string(make([]byte, 100000))) != nil {
		t.Fatal("huge key should return nil")
	}
}
