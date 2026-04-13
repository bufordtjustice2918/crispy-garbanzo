// Package security provides TLS and security hardening utilities.
package security

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// MTLSConfig holds paths for mutual TLS setup.
type MTLSConfig struct {
	CertFile string `json:"cert_file"`
	KeyFile  string `json:"key_file"`
	CAFile   string `json:"ca_file"` // CA to verify client certs
}

// NewTLSConfig builds a *tls.Config for mTLS from the given paths.
// If CAFile is set, client certificate verification is required.
func NewTLSConfig(cfg MTLSConfig) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("load server cert: %w", err)
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
	}

	if cfg.CAFile != "" {
		caPEM, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read CA cert: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("no valid certs in CA file")
		}
		tlsCfg.ClientCAs = pool
		tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return tlsCfg, nil
}

// ValidateMTLSConfig checks that all referenced files exist and are readable.
func ValidateMTLSConfig(cfg MTLSConfig) error {
	for _, f := range []struct{ name, path string }{
		{"cert", cfg.CertFile},
		{"key", cfg.KeyFile},
	} {
		if f.path == "" {
			return fmt.Errorf("%s file path is empty", f.name)
		}
		if _, err := os.Stat(f.path); err != nil {
			return fmt.Errorf("%s file %s: %w", f.name, f.path, err)
		}
	}
	if cfg.CAFile != "" {
		if _, err := os.Stat(cfg.CAFile); err != nil {
			return fmt.Errorf("CA file %s: %w", cfg.CAFile, err)
		}
	}
	return nil
}
