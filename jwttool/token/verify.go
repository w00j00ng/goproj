package token

import (
	"errors"
	"maps"
	"math/big"
	"mime"
	"strings"
	"time"
)

// Policy defines local verification requirements. Zero durations disable limits.
type Policy struct {
	Issuer, Subject, Audience, JWTID, Type string
	Required                               []string
	Leeway, MaxAge, MaxLifetime            time.Duration
}

var verificationChecks = []string{"iss", "sub", "aud", "exp", "nbf", "iat", "jti", "typ", "key_length", "max_age", "max_lifetime"}

// Verify requires an explicit algorithm and key, then checks the selected policy.
func Verify(raw string, algorithm string, key Key, policy Policy, now time.Time) (*Result, error) {
	if algorithm == "" {
		return nil, errors.New("--alg is required for verification")
	}
	if key.Secret == nil && key.PEM == nil {
		return nil, errors.New("a verification key is required")
	}
	if err := policy.validate(); err != nil {
		return nil, err
	}
	return decode(raw, algorithm, key, now, &policy)
}

func (p Policy) validate() error {
	if p.Leeway < 0 || p.MaxAge < 0 || p.MaxLifetime < 0 {
		return errors.New("verification durations must not be negative")
	}
	for _, name := range p.Required {
		switch name {
		case "iss", "sub", "aud", "exp", "nbf", "iat", "jti", "typ":
		default:
			return errors.New("required fields must be iss, sub, aud, exp, nbf, iat, jti, or typ")
		}
	}
	if p.Type != "" {
		if _, _, err := mediaType(p.Type); err != nil {
			return errors.New("expected typ must be a valid media type")
		}
	}
	return nil
}

func (p Policy) apply(result *Result, claims map[string]any, key Key, now time.Time) {
	checks := result.Validation.Checks
	if key.Secret != nil {
		checks["key_length"] = "valid"
		if len(key.Secret) < 32 {
			checks["key_length"] = "invalid"
			result.Validation.Errors = append(result.Validation.Errors, "hmac_key_too_short")
		}
	}
	required := make(map[string]bool)
	for _, name := range p.Required {
		required[name] = true
	}
	if p.MaxAge > 0 || p.MaxLifetime > 0 {
		required["iat"] = true
	}
	if p.MaxLifetime > 0 {
		required["exp"] = true
	}
	expected := map[string]string{"iss": p.Issuer, "sub": p.Subject, "aud": p.Audience, "jti": p.JWTID, "typ": p.Type}
	dates := make(map[string]*big.Rat)
	instant := numericTime(now)
	leeway := new(big.Rat).SetFrac64(int64(p.Leeway), int64(time.Second))
	claimErrors := []string{}
	fail := func(name, code string) {
		checks[name] = "invalid"
		claimErrors = append(claimErrors, code)
	}
	for _, name := range verificationChecks[:8] {
		value, exists := claims[name]
		if name == "typ" {
			value, exists = result.Header[name]
		}
		isDate := name == "exp" || name == "nbf" || name == "iat"
		if !required[name] && expected[name] == "" && !isDate {
			continue
		}
		if !exists {
			if required[name] || expected[name] != "" {
				fail(name, name+"_missing")
			}
			continue
		}
		checks[name] = "valid"
		if isDate {
			date, ok := numericDate(value)
			if !ok {
				fail(name, name+"_invalid")
				continue
			}
			dates[name] = date
			if name == "exp" {
				if instant.Cmp(new(big.Rat).Add(date, leeway)) >= 0 {
					fail(name, "token_expired")
				}
			} else if date.Cmp(new(big.Rat).Add(instant, leeway)) > 0 {
				code := "token_not_yet_valid"
				if name == "iat" {
					code = "token_issued_in_future"
				}
				fail(name, code)
			}
			continue
		}
		if name == "aud" {
			matched, valid := audienceMatches(value, expected[name])
			if !valid {
				fail(name, "aud_invalid")
			} else if !matched {
				fail(name, "aud_mismatch")
			}
			continue
		}
		text, ok := value.(string)
		if !ok || text == "" {
			fail(name, name+"_invalid")
			continue
		}
		if name == "typ" {
			actual, parameters, err := mediaType(text)
			if err != nil {
				fail(name, "typ_invalid")
				continue
			}
			if p.Type != "" {
				wanted, wantedParameters, _ := mediaType(p.Type)
				if actual != wanted || !maps.Equal(parameters, wantedParameters) {
					fail(name, "typ_mismatch")
				}
			}
		} else if expected[name] != "" && text != expected[name] {
			fail(name, name+"_mismatch")
		}
	}
	if p.MaxAge > 0 {
		checks["max_age"] = "invalid"
		if date := dates["iat"]; date != nil {
			limit := new(big.Rat).SetFrac64(int64(p.MaxAge), int64(time.Second))
			if new(big.Rat).Sub(instant, date).Cmp(limit) > 0 {
				fail("max_age", "max_age_exceeded")
			} else {
				checks["max_age"] = "valid"
			}
		}
	}
	if p.MaxLifetime > 0 {
		checks["max_lifetime"] = "invalid"
		if dates["iat"] != nil && dates["exp"] != nil {
			lifetime := new(big.Rat).Sub(dates["exp"], dates["iat"])
			limit := new(big.Rat).SetFrac64(int64(p.MaxLifetime), int64(time.Second))
			if lifetime.Sign() <= 0 {
				fail("max_lifetime", "max_lifetime_invalid")
			} else if lifetime.Cmp(limit) > 0 {
				fail("max_lifetime", "max_lifetime_exceeded")
			} else {
				checks["max_lifetime"] = "valid"
			}
		}
	}
	if len(claimErrors) != 0 {
		result.Validation.Claims = "invalid"
	}
	result.Validation.Errors = append(result.Validation.Errors, claimErrors...)
}

func audienceMatches(value any, expected string) (bool, bool) {
	var values []any
	switch audience := value.(type) {
	case string:
		values = []any{audience}
	case []any:
		values = audience
	default:
		return false, false
	}
	if len(values) == 0 {
		return false, false
	}
	matched := expected == ""
	for _, value := range values {
		audience, ok := value.(string)
		if !ok || audience == "" {
			return false, false
		}
		matched = matched || audience == expected
	}
	return matched, true
}

func mediaType(value string) (string, map[string]string, error) {
	if !strings.Contains(value, "/") {
		value = "application/" + value
	}
	return mime.ParseMediaType(value)
}
