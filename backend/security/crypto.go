package security

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/chacha20poly1305"
)

// LocalSealManager manages local secret encryption using OS-protected key storage (DPAPI on Windows)
// combined with XChaCha20-Poly1305 authenticated encryption.
type LocalSealManager struct {
	mu        sync.RWMutex
	masterKey []byte
}

// NewLocalSealManager loads or initializes a 32-byte master key protected by the OS key store.
func NewLocalSealManager(storageDir string) (*LocalSealManager, error) {
	if err := os.MkdirAll(storageDir, 0700); err != nil {
		return nil, fmt.Errorf("create storage dir: %w", err)
	}
	keyPath := filepath.Join(storageDir, "master.key.dpapi")
	var rawKey []byte

	if data, err := os.ReadFile(keyPath); err == nil && len(data) > 0 {
		unprotected, err := osUnprotectData(data)
		if err == nil && len(unprotected) == chacha20poly1305.KeySize {
			rawKey = unprotected
		}
	}

	if len(rawKey) != chacha20poly1305.KeySize {
		rawKey = make([]byte, chacha20poly1305.KeySize)
		if _, err := rand.Read(rawKey); err != nil {
			return nil, fmt.Errorf("generate master key: %w", err)
		}
		protected, err := osProtectData(rawKey)
		if err != nil {
			return nil, fmt.Errorf("protect master key: %w", err)
		}
		if err := os.WriteFile(keyPath, protected, 0600); err != nil {
			return nil, fmt.Errorf("persist master key: %w", err)
		}
	}

	return &LocalSealManager{masterKey: rawKey}, nil
}

// Encrypt encrypts plaintext with XChaCha20-Poly1305 using the OS-protected master key.
func (m *LocalSealManager) Encrypt(plaintext []byte) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	aead, err := chacha20poly1305.NewX(m.masterKey)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize(), aead.NonceSize()+len(plaintext)+aead.Overhead())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ciphertext := aead.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts base64-encoded XChaCha20-Poly1305 ciphertext.
func (m *LocalSealManager) Decrypt(encoded string) ([]byte, error) {
	if encoded == "" {
		return nil, nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	aead, err := chacha20poly1305.NewX(m.masterKey)
	if err != nil {
		return nil, err
	}
	if len(raw) < aead.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ciphertext := raw[:aead.NonceSize()], raw[aead.NonceSize():]
	return aead.Open(nil, nonce, ciphertext, nil)
}

// GenerateEd25519KeyPair returns base64-encoded public and private Ed25519 keys.
func GenerateEd25519KeyPair() (pubB64 string, privBytes []byte, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", nil, err
	}
	return base64.StdEncoding.EncodeToString(pub), priv, nil
}

// SignMessage signs a message payload using an Ed25519 private key.
func SignMessage(priv ed25519.PrivateKey, message []byte) string {
	sig := ed25519.Sign(priv, message)
	return base64.StdEncoding.EncodeToString(sig)
}

// VerifyMessageSignature verifies a base64 signature against a base64 Ed25519 public key.
func VerifyMessageSignature(pubB64 string, message []byte, sigB64 string) bool {
	pubBytes, err := base64.StdEncoding.DecodeString(pubB64)
	if err != nil || len(pubBytes) != ed25519.PublicKeySize {
		return false
	}
	sigBytes, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil || len(sigBytes) != ed25519.SignatureSize {
		return false
	}
	return ed25519.Verify(ed25519.PublicKey(pubBytes), message, sigBytes)
}

// GenerateSelfSignedTLS generates a TLS 1.3 compatible ECDSA P-256 certificate and returns
// the PEM-encoded certificate, PEM-encoded private key, and SHA-256 fingerprint.
func GenerateSelfSignedTLS(commonName string, validFor time.Duration) (certPEM []byte, keyPEM []byte, fingerprint string, err error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, "", fmt.Errorf("generate ecdsa key: %w", err)
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, "", err
	}

	notBefore := time.Now().Add(-5 * time.Minute)
	notAfter := notBefore.Add(validFor)

	ips := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok && ipnet.IP != nil {
				ips = append(ips, ipnet.IP)
			}
		}
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"FamilyControl Local LAN"},
			CommonName:   commonName,
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		IPAddresses:           ips,
		DNSNames:              []string{"localhost", commonName},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, "", fmt.Errorf("create certificate: %w", err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, nil, "", err
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})

	sum := sha256.Sum256(derBytes)
	fingerprint = hex.EncodeToString(sum[:])
	return certPEM, keyPEM, fingerprint, nil
}

// CertFingerprintFromDER computes the hex-encoded SHA-256 digest of a raw DER certificate.
func CertFingerprintFromDER(der []byte) string {
	sum := sha256.Sum256(der)
	return hex.EncodeToString(sum[:])
}

// NewPinnedTLSClientConfig creates a TLS 1.3 client configuration that pins the peer certificate SHA-256 fingerprint.
func NewPinnedTLSClientConfig(expectedFingerprint string, clientCert *tls.Certificate) *tls.Config {
	cfg := &tls.Config{
		MinVersion:         tls.VersionTLS13,
		MaxVersion:         tls.VersionTLS13,
		InsecureSkipVerify: true, // Verified explicitly via VerifyPeerCertificate pinning below
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return errors.New("no peer certificate presented")
			}
			actual := CertFingerprintFromDER(rawCerts[0])
			if expectedFingerprint != "" && actual != expectedFingerprint {
				return fmt.Errorf("certificate fingerprint mismatch: expected %s, got %s", expectedFingerprint, actual)
			}
			return nil
		},
	}
	if clientCert != nil {
		cfg.Certificates = []tls.Certificate{*clientCert}
	}
	return cfg
}

// SHA256Hex returns the hex-encoded SHA-256 hash of a string.
func SHA256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
