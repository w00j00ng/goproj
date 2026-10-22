package token_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/w00j00ng/goproj/jwttool/token"
)

func TestVerifyPolicies(t *testing.T) {
	secret := []byte(strings.Repeat("test", 8))
	now := time.Unix(100, 500000000)
	all := token.Policy{Issuer: "issuer", Subject: "alice", Audience: "api", JWTID: "id", Type: "JWT", Required: []string{"exp", "nbf", "iat"}}
	for _, tt := range []struct {
		name, header, body string
		policy             token.Policy
		codes              []string
		checks             map[string]string
	}{
		{name: "optional fields absent", body: `{}`, checks: map[string]string{"iss": "not_checked", "exp": "not_checked", "iat": "not_checked", "key_length": "valid"}},
		{name: "all fields", body: `{"iss":"issuer","sub":"alice","aud":["other","api"],"exp":101,"nbf":100.5,"iat":100,"jti":"id"}`, policy: all, checks: map[string]string{"iss": "valid", "sub": "valid", "aud": "valid", "exp": "valid", "nbf": "valid", "iat": "valid", "jti": "valid", "typ": "valid"}},
		{name: "issuer mismatch", body: `{"iss":"Issuer"}`, policy: token.Policy{Issuer: "issuer"}, codes: []string{"iss_mismatch"}},
		{name: "issuer missing", body: `{}`, policy: token.Policy{Issuer: "issuer"}, codes: []string{"iss_missing"}},
		{name: "issuer type", body: `{"iss":1}`, policy: token.Policy{Issuer: "issuer"}, codes: []string{"iss_invalid"}},
		{name: "subject mismatch", body: `{"sub":"bob"}`, policy: token.Policy{Subject: "alice"}, codes: []string{"sub_mismatch"}},
		{name: "subject missing", body: `{}`, policy: token.Policy{Subject: "alice"}, codes: []string{"sub_missing"}},
		{name: "subject type", body: `{"sub":null}`, policy: token.Policy{Subject: "alice"}, codes: []string{"sub_invalid"}},
		{name: "single audience", body: `{"aud":"api"}`, policy: token.Policy{Audience: "api"}, checks: map[string]string{"aud": "valid"}},
		{name: "audience mismatch", body: `{"aud":["other","API"]}`, policy: token.Policy{Audience: "api"}, codes: []string{"aud_mismatch"}},
		{name: "audience missing", body: `{}`, policy: token.Policy{Audience: "api"}, codes: []string{"aud_missing"}},
		{name: "audience mixed array", body: `{"aud":["api",1]}`, policy: token.Policy{Audience: "api"}, codes: []string{"aud_invalid"}},
		{name: "audience empty array", body: `{"aud":[]}`, policy: token.Policy{Audience: "api"}, codes: []string{"aud_invalid"}},
		{name: "expiration boundary", body: `{"exp":100.5}`, codes: []string{"token_expired"}, checks: map[string]string{"exp": "invalid"}},
		{name: "expiration fraction", body: `{"exp":100.500000001}`, checks: map[string]string{"exp": "valid"}},
		{name: "expiration type", body: `{"exp":"101"}`, codes: []string{"exp_invalid"}},
		{name: "not before boundary", body: `{"nbf":100.5}`, checks: map[string]string{"nbf": "valid"}},
		{name: "not before fraction", body: `{"nbf":100.500000001}`, codes: []string{"token_not_yet_valid"}},
		{name: "not before type", body: `{"nbf":null}`, codes: []string{"nbf_invalid"}},
		{name: "issued at boundary", body: `{"iat":100.5}`, checks: map[string]string{"iat": "valid"}},
		{name: "issued at fraction", body: `{"iat":100.500000001}`, codes: []string{"token_issued_in_future"}},
		{name: "issued at type", body: `{"iat":false}`, codes: []string{"iat_invalid"}},
		{name: "large numeric date", body: `{"exp":1e309}`, checks: map[string]string{"exp": "valid"}},
		{name: "JWT ID mismatch", body: `{"jti":"other"}`, policy: token.Policy{JWTID: "id"}, codes: []string{"jti_mismatch"}},
		{name: "JWT ID missing", body: `{}`, policy: token.Policy{JWTID: "id"}, codes: []string{"jti_missing"}},
		{name: "JWT ID type", body: `{"jti":[]}`, policy: token.Policy{JWTID: "id"}, codes: []string{"jti_invalid"}},
		{name: "media type normalization", header: `{"alg":"HS256","typ":"application/jWt"}`, body: `{}`, policy: token.Policy{Type: "JWT"}, checks: map[string]string{"typ": "valid"}},
		{name: "media parameters", header: `{"alg":"HS256","typ":"JWT; Foo=bar"}`, body: `{}`, policy: token.Policy{Type: "application/jwt; foo=bar"}},
		{name: "media parameter value case", header: `{"alg":"HS256","typ":"JWT; foo=Bar"}`, body: `{}`, policy: token.Policy{Type: "JWT; foo=bar"}, codes: []string{"typ_mismatch"}},
		{name: "token type mismatch", body: `{}`, policy: token.Policy{Type: "at+jwt"}, codes: []string{"typ_mismatch"}},
		{name: "payload type is not header", header: `{"alg":"HS256"}`, body: `{"typ":"JWT"}`, policy: token.Policy{Type: "JWT"}, codes: []string{"typ_missing"}},
		{name: "token type wrong type", header: `{"alg":"HS256","typ":1}`, body: `{}`, policy: token.Policy{Type: "JWT"}, codes: []string{"typ_invalid"}},
		{name: "required fields missing", body: `{}`, policy: token.Policy{Required: []string{"exp", "iat", "sub"}}, codes: []string{"sub_missing", "exp_missing", "iat_missing"}},
		{name: "required value without comparison", body: `{"jti":"unrecognized-id"}`, policy: token.Policy{Required: []string{"jti", "typ"}}, checks: map[string]string{"jti": "valid", "typ": "valid"}},
		{name: "required empty string", body: `{"sub":""}`, policy: token.Policy{Required: []string{"sub"}}, codes: []string{"sub_invalid"}},
		{name: "expiration leeway", body: `{"exp":100}`, policy: token.Policy{Leeway: time.Second}},
		{name: "expiration leeway boundary", body: `{"exp":99.5}`, policy: token.Policy{Leeway: time.Second}, codes: []string{"token_expired"}},
		{name: "not before leeway boundary", body: `{"nbf":101.5}`, policy: token.Policy{Leeway: time.Second}},
		{name: "issued at leeway boundary", body: `{"iat":101.5}`, policy: token.Policy{Leeway: time.Second}},
		{name: "issued at beyond leeway", body: `{"iat":101.500000001}`, policy: token.Policy{Leeway: time.Second}, codes: []string{"token_issued_in_future"}},
		{name: "age boundary", body: `{"iat":90.5}`, policy: token.Policy{MaxAge: 10 * time.Second}, checks: map[string]string{"max_age": "valid"}},
		{name: "age exceeded", body: `{"iat":90.499999999}`, policy: token.Policy{MaxAge: 10 * time.Second}, codes: []string{"max_age_exceeded"}},
		{name: "age does not use leeway", body: `{"iat":90}`, policy: token.Policy{MaxAge: 10 * time.Second, Leeway: time.Second}, codes: []string{"max_age_exceeded"}},
		{name: "age needs issued at", body: `{}`, policy: token.Policy{MaxAge: time.Second}, codes: []string{"iat_missing"}, checks: map[string]string{"max_age": "invalid"}},
		{name: "lifetime boundary", body: `{"iat":100,"exp":101}`, policy: token.Policy{MaxLifetime: time.Second}, checks: map[string]string{"max_lifetime": "valid"}},
		{name: "lifetime exceeded", body: `{"iat":100,"exp":101.000000001}`, policy: token.Policy{MaxLifetime: time.Second}, codes: []string{"max_lifetime_exceeded"}},
		{name: "zero lifetime", body: `{"iat":100.5,"exp":100.5}`, policy: token.Policy{MaxLifetime: time.Second}, codes: []string{"token_expired", "max_lifetime_invalid"}},
		{name: "negative lifetime", body: `{"iat":100,"exp":99}`, policy: token.Policy{MaxLifetime: time.Second}, codes: []string{"token_expired", "max_lifetime_invalid"}},
		{name: "lifetime needs dates", body: `{}`, policy: token.Policy{MaxLifetime: time.Second}, codes: []string{"exp_missing", "iat_missing"}, checks: map[string]string{"max_lifetime": "invalid"}},
		{name: "all failures reported", body: `{"iss":"other","exp":1,"iat":101}`, policy: token.Policy{Issuer: "issuer"}, codes: []string{"iss_mismatch", "token_expired", "token_issued_in_future"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			header := tt.header
			if header == "" {
				header = `{"alg":"HS256","typ":"JWT"}`
			}
			result, err := token.Verify(hmacToken(header, tt.body, secret), "HS256", token.Key{Secret: secret}, tt.policy, now)
			wantValid := len(tt.codes) == 0
			if wantValid && err != nil || !wantValid && !errors.Is(err, token.ErrValidation) {
				t.Fatalf("error=%v; want valid=%v", err, wantValid)
			}
			if result == nil || result.Validation.Valid == nil || *result.Validation.Valid != wantValid || result.Validation.Signature != "valid" || string(result.Payload) != tt.body {
				t.Fatalf("unexpected verification result: %+v", result)
			}
			if strings.Join(result.Validation.Errors, ",") != strings.Join(tt.codes, ",") || len(result.Validation.Checks) != 11 {
				t.Fatalf("validation=%+v; want errors=%v and 11 checks", result.Validation, tt.codes)
			}
			for name, status := range tt.checks {
				if result.Validation.Checks[name] != status {
					t.Fatalf("check %s=%s; want %s", name, result.Validation.Checks[name], status)
				}
			}
		})
	}
}

func TestVerifyKeyLengthAndSignature(t *testing.T) {
	for _, tt := range []struct {
		name, secret, code, signature, claims, length string
	}{
		{"31 bytes", strings.Repeat("a", 31), "hmac_key_too_short", "valid", "valid", "invalid"},
		{"32 bytes", strings.Repeat("a", 32), "", "valid", "valid", "valid"},
		{"UTF-8 bytes", strings.Repeat("가", 11), "", "valid", "valid", "valid"},
		{"wrong signature", strings.Repeat("b", 32), "signature_invalid", "invalid", "not_checked", "not_checked"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			signingKey := []byte(tt.secret)
			if tt.code == "signature_invalid" {
				signingKey = []byte(strings.Repeat("a", 32))
			}
			raw := hmacToken(`{"alg":"HS256"}`, `{"sub":"alice"}`, signingKey)
			result, err := token.Verify(raw, "HS256", token.Key{Secret: []byte(tt.secret)}, token.Policy{}, time.Unix(100, 0))
			if tt.code == "" && err != nil || tt.code != "" && !errors.Is(err, token.ErrValidation) {
				t.Fatalf("error=%v; want code %q", err, tt.code)
			}
			if result == nil || result.Validation.Signature != tt.signature || result.Validation.Claims != tt.claims || result.Validation.Checks["key_length"] != tt.length || strings.Join(result.Validation.Errors, ",") != tt.code {
				t.Fatalf("unexpected result: %+v", result)
			}
			if tt.code == "signature_invalid" {
				for name, status := range result.Validation.Checks {
					if status != "not_checked" {
						t.Fatalf("check %s ran before signature verification", name)
					}
				}
			}
		})
	}
}

func TestVerifyConfigurationErrors(t *testing.T) {
	key := token.Key{Secret: []byte(strings.Repeat("test", 8))}
	for _, tt := range []struct {
		name, algorithm string
		key             token.Key
		policy          token.Policy
	}{
		{"missing algorithm", "", key, token.Policy{}},
		{"disabled algorithm", "none", key, token.Policy{}},
		{"missing key", "HS256", token.Key{}, token.Policy{}},
		{"negative leeway", "HS256", key, token.Policy{Leeway: -time.Second}},
		{"negative age", "HS256", key, token.Policy{MaxAge: -time.Second}},
		{"negative lifetime", "HS256", key, token.Policy{MaxLifetime: -time.Second}},
		{"unknown required field", "HS256", key, token.Policy{Required: []string{"sensitive-field"}}},
		{"invalid expected media type", "HS256", key, token.Policy{Type: "sensitive-type/"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := token.Verify("sensitive-token", tt.algorithm, tt.key, tt.policy, time.Unix(100, 0))
			if result != nil || err == nil || errors.Is(err, token.ErrValidation) || strings.Contains(err.Error(), "sensitive-") {
				t.Fatalf("result=%v error=%v; want safe input error", result, err)
			}
		})
	}
}

func TestVerifyES256(t *testing.T) {
	_, public := exampleECKey(t)
	result, err := token.Verify(fixture(t, "rfc7515-es256.jwt"), "ES256", token.Key{PEM: public}, token.Policy{Issuer: "joe", Required: []string{"exp"}}, time.Unix(1300819379, 0))
	if err != nil || result == nil || result.Validation.Valid == nil || !*result.Validation.Valid {
		t.Fatalf("result=%v error=%v", result, err)
	}
	if result.Validation.Checks["key_length"] != "not_checked" || result.Validation.Checks["iss"] != "valid" || result.Validation.Checks["exp"] != "valid" {
		t.Fatalf("unexpected ES256 checks: %v", result.Validation.Checks)
	}
}
