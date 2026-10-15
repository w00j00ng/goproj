package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"

	"golang.org/x/crypto/scrypt"
	"golang.org/x/text/unicode/norm"
)

var errInvalidHash = errors.New("invalid scrypt hash: expected 32 hex salt characters and 128 hex key characters separated by ':'")

// Hash creates a password hash using Better Auth's default scrypt format.
func Hash(plain string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	saltHex := hex.EncodeToString(salt)
	key, err := deriveKey(plain, saltHex)
	if err != nil {
		return "", err
	}
	return saltHex + ":" + hex.EncodeToString(key), nil
}

// Verify reports a password match, distinguishing malformed hashes from mismatches.
func Verify(stored, plain string) (bool, error) {
	parts := strings.Split(stored, ":")
	if len(parts) != 2 || len(parts[0]) != 32 || len(parts[1]) != 128 {
		return false, errInvalidHash
	}
	if _, err := hex.DecodeString(parts[0]); err != nil {
		return false, errInvalidHash
	}
	expected, err := hex.DecodeString(parts[1])
	if err != nil {
		return false, errInvalidHash
	}
	actual, err := deriveKey(plain, parts[0])
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

func deriveKey(plain, saltHex string) ([]byte, error) {
	// Better Auth passes the hex salt as text, rather than decoding it to bytes.
	return scrypt.Key([]byte(norm.NFKC.String(plain)), []byte(saltHex), 16384, 16, 1, 64)
}
