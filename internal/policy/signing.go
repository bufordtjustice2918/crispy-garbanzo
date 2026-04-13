package policy

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// SignedBundle wraps policy rules with a signature for integrity verification.
type SignedBundle struct {
	Rules     []Rule `json:"rules"`
	Signature string `json:"signature"`
	SignedAt  string `json:"signed_at"`
	Version   int    `json:"version"`
}

// SignBundle creates a signed policy bundle from the given rules.
func SignBundle(rules []Rule, secret []byte) SignedBundle {
	data, _ := json.Marshal(rules)
	sig := hmacSHA256Hex(data, secret)
	return SignedBundle{
		Rules:     rules,
		Signature: sig,
		SignedAt:  time.Now().UTC().Format(time.RFC3339),
		Version:   1,
	}
}

// VerifyBundle checks that the bundle signature matches the rules.
func VerifyBundle(bundle SignedBundle, secret []byte) error {
	data, _ := json.Marshal(bundle.Rules)
	expected := hmacSHA256Hex(data, secret)
	if !hmac.Equal([]byte(expected), []byte(bundle.Signature)) {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}

// SaveSignedBundle writes a signed bundle to disk.
func SaveSignedBundle(path string, bundle SignedBundle) error {
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal bundle: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write bundle: %w", err)
	}
	return os.Rename(tmp, path)
}

// LoadSignedBundle reads and verifies a signed bundle from disk.
func LoadSignedBundle(path string, secret []byte) ([]Rule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read bundle %s: %w", path, err)
	}
	var bundle SignedBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return nil, fmt.Errorf("parse bundle %s: %w", path, err)
	}
	if err := VerifyBundle(bundle, secret); err != nil {
		return nil, fmt.Errorf("verify bundle %s: %w", path, err)
	}
	return bundle.Rules, nil
}

func hmacSHA256Hex(data, key []byte) string {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}
