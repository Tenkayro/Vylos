package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonTime    uint32 = 3
	argonMemory  uint32 = 64 * 1024 // 64 MB
	argonThreads uint8  = 4
	argonKeyLen  uint32 = 32
	argonSaltLen        = 16
)

var (
	ErrInvalidHash         = errors.New("invalid password hash format")
	ErrIncompatibleVersion = errors.New("incompatible argon2 version")
	ErrWeakPassword        = errors.New("password must be at least 8 characters long")
)

// HashPassword hashes a plaintext password using Argon2id and returns a standard PHC string.
func HashPassword(password string) (string, error) {
	if len(strings.TrimSpace(password)) < 8 {
		return "", ErrWeakPassword
	}
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	encoded := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads, b64Salt, b64Hash)
	return encoded, nil
}

// VerifyPassword compares a plaintext password against an Argon2id PHC string in constant time.
func VerifyPassword(password, encodedHash string) (bool, error) {
	if encodedHash == "" || password == "" {
		return false, nil
	}
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, ErrInvalidHash
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, ErrInvalidHash
	}
	if version != argon2.Version {
		return false, ErrIncompatibleVersion
	}

	var memory, timeCost uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &timeCost, &threads); err != nil {
		return false, ErrInvalidHash
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, ErrInvalidHash
	}

	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, ErrInvalidHash
	}

	computedHash := argon2.IDKey([]byte(password), salt, timeCost, memory, threads, uint32(len(expectedHash)))
	if subtle.ConstantTimeCompare(computedHash, expectedHash) == 1 {
		return true, nil
	}
	return false, nil
}

const pairingAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"

// GeneratePairingCode creates a human-readable, high-entropy one-time pairing code (e.g., "K7M9-P4X2").
func GeneratePairingCode() (string, error) {
	b := make([]byte, 8)
	max := big.NewInt(int64(len(pairingAlphabet)))
	for i := range b {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b[i] = pairingAlphabet[n.Int64()]
	}
	return fmt.Sprintf("%s-%s", string(b[:4]), string(b[4:])), nil
}

// GenerateRecoveryCode generates an emergency parent recovery key (e.g., "FC-XXXX-XXXX-XXXX-XXXX").
func GenerateRecoveryCode() (string, error) {
	b := make([]byte, 16)
	max := big.NewInt(int64(len(pairingAlphabet)))
	for i := range b {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b[i] = pairingAlphabet[n.Int64()]
	}
	return fmt.Sprintf("FC-%s-%s-%s-%s", string(b[0:4]), string(b[4:8]), string(b[8:12]), string(b[12:16])), nil
}

// NormalizeCode strips spaces and hyphens and uppercases a pairing or recovery code.
func NormalizeCode(code string) string {
	cleaned := strings.ToUpper(strings.TrimSpace(code))
	cleaned = strings.ReplaceAll(cleaned, "-", "")
	cleaned = strings.ReplaceAll(cleaned, " ", "")
	return cleaned
}
