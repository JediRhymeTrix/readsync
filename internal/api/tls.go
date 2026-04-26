// internal/api/tls.go
//
// Self-signed cert generator. The admin UI is local-only by default
// and runs over plain HTTP, but users (or installations behind a
// reverse proxy) can opt into HTTPS via Deps.TLSCert.
//
// The generated cert covers 127.0.0.1 + localhost only; any LAN
// reader endpoint terminates TLS in its own adapter.

package api

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"time"
)

// GenerateSelfSigned creates a fresh TLS certificate suitable for
// 127.0.0.1 + localhost. The cert is valid for `validity` days.
//
// If certPath / keyPath are non-empty, the PEM-encoded cert and key
// are also written to disk so the user can re-trust them.
func GenerateSelfSigned(certPath, keyPath string, validity time.Duration) (*tls.Certificate, error) {
	if validity == 0 {
		validity = 365 * 24 * time.Hour
	}
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("api.tls: generate key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("api.tls: serial: %w", err)
	}
	tmpl := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			Organization: []string{"ReadSync"},
			CommonName:   "ReadSync Admin UI",
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(validity),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		return nil, fmt.Errorf("api.tls: create cert: %w", err)
	}

	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("api.tls: marshal key: %w", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	if certPath != "" {
		if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
			return nil, fmt.Errorf("api.tls: write cert: %w", err)
		}
	}
	if keyPath != "" {
		if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
			return nil, fmt.Errorf("api.tls: write key: %w", err)
		}
	}

	tc, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("api.tls: x509 keypair: %w", err)
	}
	return &tc, nil
}

// LoadOrGenerateTLS loads cert+key PEM files, or generates a new self-
// signed pair if either file is missing. Convenient for first-run.
func LoadOrGenerateTLS(certPath, keyPath string) (*tls.Certificate, error) {
	if _, err := os.Stat(certPath); err == nil {
		if _, err := os.Stat(keyPath); err == nil {
			tc, err := tls.LoadX509KeyPair(certPath, keyPath)
			if err == nil {
				return &tc, nil
			}
		}
	}
	return GenerateSelfSigned(certPath, keyPath, 0)
}
