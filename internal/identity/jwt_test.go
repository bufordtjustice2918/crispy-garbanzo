package identity

import (
	"testing"
	"time"
)

func TestSignAndVerify(t *testing.T) {
	secret := []byte("test-secret-key-32bytes-long!!!!!")
	claims := JWTClaims{
		AgentID:     "agent-1",
		TeamID:      "team-1",
		ProjectID:   "proj-1",
		Environment: "test",
		Iat:         time.Now().Unix(),
		Exp:         time.Now().Add(time.Hour).Unix(),
	}

	token := SignJWT(claims, secret)
	got, err := VerifyJWT(token, secret)
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if got.AgentID != "agent-1" {
		t.Fatalf("want agent-1, got %s", got.AgentID)
	}
	if got.TeamID != "team-1" {
		t.Fatalf("want team-1, got %s", got.TeamID)
	}
}

func TestVerifyBadSignature(t *testing.T) {
	secret := []byte("correct-secret")
	claims := JWTClaims{AgentID: "a1", Exp: time.Now().Add(time.Hour).Unix()}
	token := SignJWT(claims, secret)

	_, err := VerifyJWT(token, []byte("wrong-secret"))
	if err == nil {
		t.Fatal("should reject bad signature")
	}
}

func TestVerifyExpired(t *testing.T) {
	secret := []byte("secret")
	claims := JWTClaims{AgentID: "a1", Exp: time.Now().Add(-time.Hour).Unix()}
	token := SignJWT(claims, secret)

	_, err := VerifyJWT(token, secret)
	if err == nil {
		t.Fatal("should reject expired token")
	}
}

func TestVerifyMissingAgentID(t *testing.T) {
	secret := []byte("secret")
	claims := JWTClaims{Exp: time.Now().Add(time.Hour).Unix()}
	token := SignJWT(claims, secret)

	_, err := VerifyJWT(token, secret)
	if err == nil {
		t.Fatal("should reject missing agent_id")
	}
}

func TestVerifyMalformed(t *testing.T) {
	_, err := VerifyJWT("not.a.valid.jwt.at.all", []byte("s"))
	if err == nil {
		t.Fatal("should reject malformed")
	}
	_, err = VerifyJWT("onlyonepart", []byte("s"))
	if err == nil {
		t.Fatal("should reject single part")
	}
}
