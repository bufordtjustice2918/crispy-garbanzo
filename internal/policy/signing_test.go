package policy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSignAndVerify(t *testing.T) {
	secret := []byte("test-signing-secret")
	rules := []Rule{
		{PolicyID: "p1", AgentID: "*", Domains: []string{"example.com"}, Action: "allow"},
	}

	bundle := SignBundle(rules, secret)
	if bundle.Signature == "" {
		t.Fatal("signature should not be empty")
	}
	if err := VerifyBundle(bundle, secret); err != nil {
		t.Fatalf("verify failed: %v", err)
	}
}

func TestVerifyBadSecret(t *testing.T) {
	rules := []Rule{{PolicyID: "p1", AgentID: "*", Domains: []string{"x"}, Action: "allow"}}
	bundle := SignBundle(rules, []byte("correct"))

	if err := VerifyBundle(bundle, []byte("wrong")); err == nil {
		t.Fatal("should reject bad secret")
	}
}

func TestVerifyTampered(t *testing.T) {
	secret := []byte("s")
	rules := []Rule{{PolicyID: "p1", AgentID: "*", Domains: []string{"x"}, Action: "allow"}}
	bundle := SignBundle(rules, secret)

	// Tamper with rules after signing.
	bundle.Rules[0].Action = "deny"
	if err := VerifyBundle(bundle, secret); err == nil {
		t.Fatal("should detect tampering")
	}
}

func TestSaveAndLoadBundle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.signed.json")
	secret := []byte("secret")

	rules := []Rule{
		{PolicyID: "p1", AgentID: "a1", Domains: []string{"good.com"}, Action: "allow"},
		{PolicyID: "p2", AgentID: "*", Domains: []string{"*"}, Action: "deny"},
	}
	bundle := SignBundle(rules, secret)
	if err := SaveSignedBundle(path, bundle); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadSignedBundle(path, secret)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 2 {
		t.Fatalf("want 2 rules, got %d", len(loaded))
	}
	if loaded[0].PolicyID != "p1" {
		t.Fatalf("want p1, got %s", loaded[0].PolicyID)
	}
}

func TestLoadBundleBadSignature(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.signed.json")

	rules := []Rule{{PolicyID: "p1", AgentID: "*", Domains: []string{"x"}, Action: "allow"}}
	bundle := SignBundle(rules, []byte("secret1"))
	SaveSignedBundle(path, bundle)

	_, err := LoadSignedBundle(path, []byte("secret2"))
	if err == nil {
		t.Fatal("should reject bad signature on load")
	}
}

func TestLoadBundleMissing(t *testing.T) {
	_, err := LoadSignedBundle("/nonexistent", []byte("s"))
	if err == nil {
		t.Fatal("should error on missing file")
	}
}

func TestSaveLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bundle.json")
	secret := []byte("roundtrip-key")

	rules := []Rule{
		{PolicyID: "allow-all", AgentID: "*", Domains: []string{"*"}, Action: "allow"},
	}

	// Sign, save, load, verify — full roundtrip.
	bundle := SignBundle(rules, secret)
	SaveSignedBundle(path, bundle)

	data, _ := os.ReadFile(path)
	if len(data) == 0 {
		t.Fatal("file should not be empty")
	}

	loaded, err := LoadSignedBundle(path, secret)
	if err != nil {
		t.Fatal(err)
	}
	if loaded[0].PolicyID != "allow-all" {
		t.Fatal("roundtrip lost data")
	}
}
