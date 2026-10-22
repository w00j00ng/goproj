package es256

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/pem"
	"errors"

	"github.com/golang-jwt/jwt/v5"
)

type Algorithm struct{}

func (Algorithm) SigningMethod() jwt.SigningMethod { return jwt.SigningMethodES256 }

func (Algorithm) UsesSecret() bool { return false }

func (Algorithm) ParseKey(data []byte, signing bool) (any, error) {
	invalid := errors.New("key must be an unencrypted P-256 PEM key of the required type")
	trimmed := bytes.TrimSpace(data)
	if bytes.Count(trimmed, []byte("-----BEGIN ")) != 1 {
		return nil, invalid
	}
	block, rest := pem.Decode(trimmed)
	if block == nil || len(bytes.TrimSpace(rest)) != 0 || len(block.Headers) != 0 ||
		!bytes.HasPrefix(trimmed, []byte("-----BEGIN "+block.Type+"-----")) {
		return nil, invalid
	}
	if !signing {
		if block.Type != "PUBLIC KEY" {
			return nil, invalid
		}
		parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
		public, ok := parsed.(*ecdsa.PublicKey)
		if err != nil || !ok || public.Curve != elliptic.P256() {
			return nil, invalid
		}
		return public, nil
	}
	var parsed any
	var err error
	switch block.Type {
	case "EC PRIVATE KEY":
		parsed, err = x509.ParseECPrivateKey(block.Bytes)
	case "PRIVATE KEY":
		parsed, err = x509.ParsePKCS8PrivateKey(block.Bytes)
	default:
		return nil, invalid
	}
	private, ok := parsed.(*ecdsa.PrivateKey)
	if err != nil || !ok || private.Curve != elliptic.P256() {
		return nil, invalid
	}
	return private, nil
}
