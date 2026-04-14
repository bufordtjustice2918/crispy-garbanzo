package identity

import "testing"

// FuzzVerifyJWT throws random bytes at the JWT verifier.
// Must never panic — only return errors.
func FuzzVerifyJWT(f *testing.F) {
	// Seed corpus with valid and near-valid tokens.
	secret := []byte("fuzz-secret")
	validToken := SignJWT(JWTClaims{AgentID: "fuzz-agent", Exp: 9999999999}, secret)

	f.Add(validToken)
	f.Add("not.a.jwt")
	f.Add("")
	f.Add("eyJhbGciOiJIUzI1NiJ9.eyJhZ2VudF9pZCI6IngiLCJleHAiOjF9.badsig")
	f.Add("a]]]].b.c")
	f.Add(string(make([]byte, 10000))) // oversized

	f.Fuzz(func(t *testing.T, token string) {
		// Must not panic. Errors are fine.
		_, _ = VerifyJWT(token, secret)
	})
}

// FuzzBase64URLDecode throws random strings at the decoder.
func FuzzBase64URLDecode(f *testing.F) {
	f.Add("SGVsbG8")
	f.Add("")
	f.Add("!!!invalid!!!")
	f.Add(string(make([]byte, 5000)))

	f.Fuzz(func(t *testing.T, s string) {
		_, _ = base64URLDecode(s)
	})
}
