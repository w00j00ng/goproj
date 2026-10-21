package token

import (
	"errors"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/w00j00ng/goproj/jwttool/algorithm/es256"
	"github.com/w00j00ng/goproj/jwttool/algorithm/hs256"
)

type algorithm interface {
	SigningMethod() jwt.SigningMethod
	UsesSecret() bool
	ParseKey(data []byte, signing bool) (any, error)
}

var algorithms = []algorithm{
	hs256.Algorithm{},
	es256.Algorithm{},
}

// SupportedAlgorithms returns the enabled signing and verification algorithms.
func SupportedAlgorithms() []string {
	names := make([]string, len(algorithms))
	for i, implementation := range algorithms {
		names[i] = implementation.SigningMethod().Alg()
	}
	return names
}

func findAlgorithm(name string) algorithm {
	for _, implementation := range algorithms {
		if implementation.SigningMethod().Alg() == name {
			return implementation
		}
	}
	return nil
}

func unsupportedAlgorithm() error {
	return errors.New("algorithm must be one of: " + strings.Join(SupportedAlgorithms(), ", "))
}

func validateKey(key Key) error {
	if key.Secret != nil && key.PEM != nil {
		return errors.New("secret and PEM key cannot be combined")
	}
	if len(key.Secret) > MaxKeySize || len(key.PEM) > MaxKeySize {
		return errors.New("key exceeds size limit")
	}
	if key.Secret != nil && len(key.Secret) == 0 {
		return errors.New("secret must not be empty")
	}
	if key.PEM != nil && len(key.PEM) == 0 {
		return errors.New("PEM key must not be empty")
	}
	return nil
}

func parseKey(implementation algorithm, key Key, signing bool) (any, error) {
	data := key.PEM
	if implementation.UsesSecret() {
		data = key.Secret
	}
	if data == nil {
		return nil, errors.New("algorithm requires a matching key input")
	}
	return implementation.ParseKey(data, signing)
}
