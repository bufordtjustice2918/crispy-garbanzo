package security

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func generateTestCert(t *testing.T, dir, name string) (certPath, keyPath string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPath = filepath.Join(dir, name+".crt")
	keyPath = filepath.Join(dir, name+".key")

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	os.WriteFile(certPath, certPEM, 0o644)

	keyDER, _ := x509.MarshalECPrivateKey(key)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	os.WriteFile(keyPath, keyPEM, 0o600)

	return certPath, keyPath
}

func TestNewTLSConfig(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath := generateTestCert(t, dir, "server")

	cfg := MTLSConfig{CertFile: certPath, KeyFile: keyPath}
	tlsCfg, err := NewTLSConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(tlsCfg.Certificates) != 1 {
		t.Fatal("expected 1 certificate")
	}
	if tlsCfg.MinVersion != 0x0303 { // TLS 1.2
		t.Fatal("expected TLS 1.2 minimum")
	}
}

func TestNewTLSConfigWithCA(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath := generateTestCert(t, dir, "server")
	caPath, _ := generateTestCert(t, dir, "ca")

	cfg := MTLSConfig{CertFile: certPath, KeyFile: keyPath, CAFile: caPath}
	tlsCfg, err := NewTLSConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if tlsCfg.ClientCAs == nil {
		t.Fatal("expected ClientCAs pool")
	}
	if tlsCfg.ClientAuth != 4 { // RequireAndVerifyClientCert
		t.Fatalf("expected RequireAndVerifyClientCert, got %d", tlsCfg.ClientAuth)
	}
}

func TestNewTLSConfigMissingCert(t *testing.T) {
	_, err := NewTLSConfig(MTLSConfig{CertFile: "/nonexistent", KeyFile: "/nonexistent"})
	if err == nil {
		t.Fatal("should fail with missing cert")
	}
}

func TestValidateMTLSConfig(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath := generateTestCert(t, dir, "test")

	// Valid config.
	err := ValidateMTLSConfig(MTLSConfig{CertFile: certPath, KeyFile: keyPath})
	if err != nil {
		t.Fatal(err)
	}

	// Missing cert.
	err = ValidateMTLSConfig(MTLSConfig{CertFile: "/nonexistent", KeyFile: keyPath})
	if err == nil {
		t.Fatal("should fail")
	}

	// Empty path.
	err = ValidateMTLSConfig(MTLSConfig{CertFile: "", KeyFile: keyPath})
	if err == nil {
		t.Fatal("should fail on empty path")
	}
}
