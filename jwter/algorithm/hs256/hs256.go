package hs256

import (
	"errors"

	"github.com/golang-jwt/jwt/v5"
)

type Algorithm struct{}

func (Algorithm) SigningMethod() jwt.SigningMethod { return jwt.SigningMethodHS256 }

func (Algorithm) UsesSecret() bool { return true }

func (Algorithm) ParseKey(data []byte, _ bool) (any, error) {
	if len(data) == 0 {
		return nil, errors.New("secret must not be empty")
	}
	return data, nil
}
