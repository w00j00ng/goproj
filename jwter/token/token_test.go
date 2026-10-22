package token_test

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/w00j00ng/goproj/jwter/token"
)

func TestSupportedAlgorithms(t *testing.T) {
	want := []string{"HS256", "ES256"}
	got := token.SupportedAlgorithms()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("algorithms=%v; want %v", got, want)
	}
	got[0] = "RS256"
	if !reflect.DeepEqual(token.SupportedAlgorithms(), want) {
		t.Fatal("caller changed the enabled algorithms")
	}
	for _, name := range []string{"RS256", "EdDSA", "none"} {
		t.Run(name, func(t *testing.T) {
			if _, err := token.Encode([]byte(`{}`), name, token.Key{Secret: []byte("secret")}); err == nil {
				t.Fatal("encoding accepted an algorithm that is not enabled")
			}
			raw := compact(`{"alg":"`+name+`"}`, `{}`, "AQ")
			if _, err := token.Decode(raw, "", token.Key{}, time.Unix(100, 0)); err != nil {
				t.Fatalf("keyless inspection should still work: %v", err)
			}
			result, err := token.Decode(raw, "", token.Key{Secret: []byte("secret")}, time.Unix(100, 0))
			if !errors.Is(err, token.ErrValidation) {
				t.Fatalf("error=%v; want verification rejection", err)
			}
			assertValidation(t, result, "invalid", "not_checked", boolPointer(false), []string{"algorithm_mismatch"})
		})
	}
}

func TestDecodePublishedTokens(t *testing.T) {
	_, public := exampleECKey(t)
	hmacKey, err := base64.RawURLEncoding.DecodeString(fixture(t, "rfc7515-hmac-key.b64url"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name, file, algorithm string
		key                   token.Key
	}{
		{"HS256 explicit", "rfc7515-hs256.jwt", "HS256", token.Key{Secret: hmacKey}},
		{"HS256 inferred", "rfc7515-hs256.jwt", "", token.Key{Secret: hmacKey}},
		{"ES256 explicit", "rfc7515-es256.jwt", "ES256", token.Key{PEM: public}},
		{"ES256 inferred", "rfc7515-es256.jwt", "", token.Key{PEM: public}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := token.Decode(fixture(t, tt.file), tt.algorithm, tt.key, time.Unix(1300819379, 0))
			if err != nil {
				t.Fatal(err)
			}
			assertValidation(t, got, "valid", "valid", boolPointer(true), nil)
			if string(got.Payload) != "{\"iss\":\"joe\",\r\n \"exp\":1300819380,\r\n \"http://example.com/is_root\":true}" {
				t.Fatalf("original published payload not retained: %s", got.Payload)
			}
		})
	}
}

func TestDecodeWithoutKey(t *testing.T) {
	for _, tt := range []struct {
		name, header, payload, signature, algorithm string
	}{
		{"unsigned", `{"alg":"none"}`, `{"exp":0}`, "", ""},
		{"unknown algorithm", `{"alg":"custom"}`, `{"exp":null}`, "", ""},
		{"specified algorithm", `{"alg":"HS256"}`, `{"nbf":"later"}`, "AQ", "HS256"},
		{"signed token", `{"alg":"ES256"}`, `{"large":9007199254740993}`, "AQ", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			raw := compact(tt.header, tt.payload, tt.signature)
			got, err := token.Decode(raw, tt.algorithm, token.Key{}, time.Unix(100, 0))
			if err != nil {
				t.Fatal(err)
			}
			assertValidation(t, got, "not_checked", "not_checked", nil, nil)
			if string(got.Payload) != tt.payload {
				t.Fatalf("payload = %s, want %s", got.Payload, tt.payload)
			}
			encoded, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(encoded), `"valid":null,"errors":[]`) {
				t.Fatalf("unknown validation must be explicit JSON null and empty array: %s", encoded)
			}
		})
	}
}

func TestDecodeTrimsOuterWhitespace(t *testing.T) {
	for _, whitespace := range []string{"\n", " \t\r\n"} {
		t.Run(base64.RawURLEncoding.EncodeToString([]byte(whitespace)), func(t *testing.T) {
			raw := whitespace + compact(`{"alg":"none"}`, `{}`, "") + whitespace
			got, err := token.Decode(raw, "", token.Key{}, time.Unix(100, 0))
			if err != nil {
				t.Fatal(err)
			}
			assertValidation(t, got, "not_checked", "not_checked", nil, nil)
		})
	}
}

func TestDecodeValidation(t *testing.T) {
	secret := []byte("validation-test-secret")
	now := time.Unix(100, 500000000)
	for _, tt := range []struct {
		name, header, payload, signature, claims string
		key                                      []byte
		codes                                    []string
	}{
		{"no dates", `{"alg":"HS256"}`, `{}`, "valid", "valid", secret, nil},
		{"large header number", `{"alg":"HS256","custom":1e309}`, `{}`, "valid", "valid", secret, nil},
		{"expiration in future", `{"alg":"HS256"}`, `{"exp":101}`, "valid", "valid", secret, nil},
		{"expiration boundary", `{"alg":"HS256"}`, `{"exp":100.5}`, "valid", "invalid", secret, []string{"token_expired"}},
		{"fractional expiration future", `{"alg":"HS256"}`, `{"exp":100.500000001}`, "valid", "valid", secret, nil},
		{"expired", `{"alg":"HS256"}`, `{"exp":99}`, "valid", "invalid", secret, []string{"token_expired"}},
		{"not before boundary", `{"alg":"HS256"}`, `{"nbf":100.5}`, "valid", "valid", secret, nil},
		{"not before future", `{"alg":"HS256"}`, `{"nbf":100.500000001}`, "valid", "invalid", secret, []string{"token_not_yet_valid"}},
		{"both dates invalid", `{"alg":"HS256"}`, `{"exp":100,"nbf":101}`, "valid", "invalid", secret, []string{"token_expired", "token_not_yet_valid"}},
		{"iat ignored", `{"alg":"HS256"}`, `{"iat":"not-a-date"}`, "valid", "valid", secret, nil},
		{"future iat ignored", `{"alg":"HS256"}`, `{"iat":9007199254740993}`, "valid", "valid", secret, nil},
		{"large expiration", `{"alg":"HS256"}`, `{"exp":9007199254740993}`, "valid", "valid", secret, nil},
		{"null expiration", `{"alg":"HS256"}`, `{"exp":null}`, "valid", "invalid", secret, []string{"claim_type_invalid"}},
		{"string expiration", `{"alg":"HS256"}`, `{"exp":"101"}`, "valid", "invalid", secret, []string{"claim_type_invalid"}},
		{"bool expiration", `{"alg":"HS256"}`, `{"exp":true}`, "valid", "invalid", secret, []string{"claim_type_invalid"}},
		{"null not before", `{"alg":"HS256"}`, `{"nbf":null}`, "valid", "invalid", secret, []string{"claim_type_invalid"}},
		{"object not before", `{"alg":"HS256"}`, `{"nbf":{}}`, "valid", "invalid", secret, []string{"claim_type_invalid"}},
		{"bad signature skips expired claims", `{"alg":"HS256"}`, `{"exp":0}`, "invalid", "not_checked", []byte("wrong-secret"), []string{"signature_invalid"}},
		{"algorithm mismatch skips claims", `{"alg":"HS384"}`, `{"exp":null}`, "invalid", "not_checked", secret, []string{"algorithm_mismatch"}},
		{"none rejected with key", `{"alg":"none"}`, `{}`, "invalid", "not_checked", secret, []string{"algorithm_mismatch"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			raw := hmacToken(tt.header, tt.payload, secret)
			got, err := token.Decode(raw, "", token.Key{Secret: tt.key}, now)
			wantValid := len(tt.codes) == 0
			if wantValid && err != nil || !wantValid && !errors.Is(err, token.ErrValidation) {
				t.Fatalf("error = %v, validation failure wanted = %v", err, !wantValid)
			}
			assertValidation(t, got, tt.signature, tt.claims, boolPointer(wantValid), tt.codes)
			if string(got.Payload) != tt.payload {
				t.Fatalf("validation result lost payload: %s", got.Payload)
			}
		})
	}
}

func TestDecodeRejectsMalformedInput(t *testing.T) {
	valid := compact(`{"alg":"HS256"}`, `{}`, "AQ")
	for _, tt := range []struct{ name, raw, algorithm string }{
		{"empty", "", ""},
		{"two segments", "e30.e30", ""},
		{"four segments", valid + ".AQ", ""},
		{"empty header", ".e30.", ""},
		{"invalid base64", "++.e30.AQ", ""},
		{"padded base64", "e30=.e30.AQ", ""},
		{"base64 newline", "e3\n0.e30.AQ", ""},
		{"noncanonical base64", "e31.e30.AQ", ""},
		{"invalid signature base64", compact(`{"alg":"HS256"}`, `{}`, "?"), ""},
		{"header array", compact(`[]`, `{}`, ""), ""},
		{"payload array", compact(`{"alg":"HS256"}`, `[]`, ""), ""},
		{"payload null", compact(`{"alg":"HS256"}`, `null`, ""), ""},
		{"trailing JSON", compact(`{"alg":"HS256"}`, `{} {}`, ""), ""},
		{"invalid UTF8", compact(`{"alg":"HS256"}`, "{\"s\":\"\xff\"}", ""), ""},
		{"header duplicate", compact(`{"alg":"HS256","alg":"HS256"}`, `{}`, ""), ""},
		{"escaped header duplicate", compact(`{"alg":"HS256","\u0061lg":"none"}`, `{}`, ""), ""},
		{"payload duplicate", compact(`{"alg":"HS256"}`, `{"exp":1,"exp":2}`, ""), ""},
		{"empty algorithm", compact(`{"alg":""}`, `{}`, ""), ""},
		{"missing algorithm", compact(`{}`, `{}`, ""), ""},
		{"numeric algorithm", compact(`{"alg":1}`, `{}`, ""), ""},
		{"critical extension", compact(`{"alg":"HS256","crit":["custom"],"custom":true}`, `{}`, ""), ""},
		{"malformed critical extension", compact(`{"alg":"HS256","crit":true}`, `{}`, ""), ""},
		{"unencoded payload", compact(`{"alg":"HS256","b64":false}`, `{}`, ""), ""},
		{"malformed b64 option", compact(`{"alg":"HS256","b64":"true"}`, `{}`, ""), ""},
		{"input limit", strings.Repeat("a", token.MaxInputSize+1), ""},
		{"unsupported explicit algorithm", valid, "HS384"},
		{"keyless algorithm mismatch", valid, "ES256"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := token.Decode(tt.raw, tt.algorithm, token.Key{}, time.Unix(100, 0))
			if err == nil || errors.Is(err, token.ErrValidation) || got != nil {
				t.Fatalf("result = %v, error = %v; want ordinary input error", got, err)
			}
		})
	}
}

func TestDecodeRejectsInvalidKeyOptions(t *testing.T) {
	_, public := exampleECKey(t)
	for _, tt := range []struct {
		name, algorithm string
		key             token.Key
	}{
		{"empty secret", "", token.Key{Secret: []byte{}}},
		{"empty PEM", "", token.Key{PEM: []byte{}}},
		{"mixed keys", "", token.Key{Secret: []byte("secret"), PEM: public}},
		{"mixed empty keys", "", token.Key{Secret: []byte{}, PEM: []byte{}}},
		{"oversized secret", "", token.Key{Secret: make([]byte, token.MaxKeySize+1)}},
		{"secret for ECDSA", "ES256", token.Key{Secret: []byte("secret")}},
		{"public key for HMAC", "HS256", token.Key{PEM: public}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := token.Decode(fixture(t, "rfc7515-hs256.jwt"), tt.algorithm, tt.key, time.Unix(1300819379, 0))
			if err == nil || errors.Is(err, token.ErrValidation) || got != nil {
				t.Fatalf("result = %v, error = %v; want key option error", got, err)
			}
		})
	}
}

func TestDecodeES256ValidationFailures(t *testing.T) {
	_, public := exampleECKey(t)
	raw := fixture(t, "rfc7515-es256.jwt")
	parts := strings.Split(raw, ".")
	badSignature := decodeSegment(t, parts[2])
	badSignature[0] ^= 1
	for _, tt := range []struct{ name, raw, code string }{
		{"tampered signature", parts[0] + "." + parts[1] + "." + base64.RawURLEncoding.EncodeToString(badSignature), "signature_invalid"},
		{"short signature", parts[0] + "." + parts[1] + ".AQ", "signature_invalid"},
		{"HMAC header with ECDSA key", fixture(t, "rfc7515-hs256.jwt"), "algorithm_mismatch"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := token.Decode(tt.raw, "", token.Key{PEM: public}, time.Unix(1300819381, 0))
			if !errors.Is(err, token.ErrValidation) {
				t.Fatalf("error = %v, want validation failure", err)
			}
			assertValidation(t, got, "invalid", "not_checked", boolPointer(false), []string{tt.code})
		})
	}
}

func TestEncodePreservesClaimsAndProducesVerifiableSignatures(t *testing.T) {
	private, _ := exampleECKey(t)
	sec1, err := x509.MarshalECPrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	pkcs8, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name, algorithm string
		key             token.Key
	}{
		{"HS256 raw secret", "HS256", token.Key{Secret: []byte("raw-secret\n")}},
		{"ES256 SEC1", "ES256", token.Key{PEM: pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: sec1})}},
		{"ES256 PKCS8", "ES256", token.Key{PEM: pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8})}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := token.Encode([]byte(`{"sub":"alice","large":9007199254740993}`), tt.algorithm, tt.key)
			if err != nil {
				t.Fatal(err)
			}
			parts := strings.Split(encoded, ".")
			if len(parts) != 3 {
				t.Fatalf("compact token has %d segments", len(parts))
			}
			header := decodeSegment(t, parts[0])
			if string(header) != `{"alg":"`+tt.algorithm+`","typ":"JWT"}` {
				t.Fatalf("header = %s", header)
			}
			if payload := decodeSegment(t, parts[1]); string(payload) != `{"large":9007199254740993,"sub":"alice"}` {
				t.Fatalf("claims changed: %s", payload)
			}
			signature := decodeSegment(t, parts[2])
			if tt.algorithm == "HS256" {
				mac := hmac.New(sha256.New, tt.key.Secret)
				_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
				if !hmac.Equal(signature, mac.Sum(nil)) {
					t.Fatal("independent HMAC verifier rejected signature")
				}
			} else {
				digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
				if len(signature) != 64 || !ecdsa.Verify(&private.PublicKey, digest[:], new(big.Int).SetBytes(signature[:32]), new(big.Int).SetBytes(signature[32:])) {
					t.Fatal("independent ECDSA verifier rejected signature")
				}
			}
		})
	}
}

func TestEncodePreservesNestedClaims(t *testing.T) {
	for _, tt := range []struct{ name, body, expected string }{
		{
			"nested Korean claims and large integer",
			`{"profile":{"roles":["사용자","관리자"],"name":"홍길동","large":9007199254740993},"values":[null,true,{"label":"한글"}]}`,
			`{"profile":{"large":9007199254740993,"name":"홍길동","roles":["사용자","관리자"]},"values":[null,true,{"label":"한글"}]}`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			key := token.Key{Secret: []byte("nested-claims-test-secret")}
			encoded, err := token.Encode([]byte(tt.body), "HS256", key)
			if err != nil {
				t.Fatal(err)
			}
			got, err := token.Decode(encoded, "HS256", key, time.Unix(100, 0))
			if err != nil {
				t.Fatal(err)
			}
			if string(got.Payload) != tt.expected {
				t.Fatalf("claims = %s, want %s", got.Payload, tt.expected)
			}
		})
	}
}

func TestEncodeRejectsInvalidInput(t *testing.T) {
	_, public := exampleECKey(t)
	for _, tt := range []struct {
		name, body, algorithm string
		key                   token.Key
	}{
		{"algorithm required", `{}`, "", token.Key{Secret: []byte("secret")}},
		{"unsupported algorithm", `{}`, "none", token.Key{Secret: []byte("secret")}},
		{"missing secret", `{}`, "HS256", token.Key{}},
		{"empty secret", `{}`, "HS256", token.Key{Secret: []byte{}}},
		{"empty PEM", `{}`, "ES256", token.Key{PEM: []byte{}}},
		{"array", `[]`, "HS256", token.Key{Secret: []byte("secret")}},
		{"null", `null`, "HS256", token.Key{Secret: []byte("secret")}},
		{"trailing JSON", `{} {}`, "HS256", token.Key{Secret: []byte("secret")}},
		{"duplicate claim", `{"sub":"a","sub":"b"}`, "HS256", token.Key{Secret: []byte("secret")}},
		{"invalid UTF8", "{\"s\":\"\xff\"}", "HS256", token.Key{Secret: []byte("secret")}},
		{"input limit", strings.Repeat("a", token.MaxInputSize+1), "HS256", token.Key{Secret: []byte("secret")}},
		{"secret limit", `{}`, "HS256", token.Key{Secret: make([]byte, token.MaxKeySize+1)}},
		{"mixed keys", `{}`, "HS256", token.Key{Secret: []byte("secret"), PEM: public}},
		{"wrong HMAC key kind", `{}`, "HS256", token.Key{PEM: public}},
		{"wrong ECDSA key kind", `{}`, "ES256", token.Key{Secret: []byte("secret")}},
		{"public cannot sign", `{}`, "ES256", token.Key{PEM: public}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := token.Encode([]byte(tt.body), tt.algorithm, tt.key)
			if err == nil || encoded != "" || errors.Is(err, token.ErrValidation) {
				t.Fatalf("token = %q, error = %v; want ordinary input error", encoded, err)
			}
		})
	}
}

func TestRejectsInvalidPEMKeys(t *testing.T) {
	private, public := exampleECKey(t)
	sec1, err := x509.MarshalECPrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	wrongCurve, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	wrongPrivate, err := x509.MarshalECPrivateKey(wrongCurve)
	if err != nil {
		t.Fatal(err)
	}
	wrongPublic, err := x509.MarshalPKIXPublicKey(&wrongCurve.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	otherPublic, err := x509.MarshalPKIXPublicKey(ed25519.PublicKey(make([]byte, ed25519.PublicKeySize)))
	if err != nil {
		t.Fatal(err)
	}
	privatePEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: sec1})
	for _, tt := range []struct {
		name string
		pem  []byte
	}{
		{"malformed", []byte("sensitive-PEM-content")},
		{"trailing junk", append(append([]byte{}, public...), []byte("junk")...)},
		{"leading junk", append([]byte("junk\n"), public...)},
		{"malformed public block before valid key", append([]byte("-----BEGIN PUBLIC KEY-----\nmalformed-leading-block\n"), public...)},
		{"malformed private block before valid key", append([]byte("-----BEGIN EC PRIVATE KEY-----\nmalformed-leading-block\n"), privatePEM...)},
		{"multiple blocks", append(append([]byte{}, public...), public...)},
		{"encrypted", pem.EncodeToMemory(&pem.Block{Type: "ENCRYPTED PRIVATE KEY", Bytes: sec1})},
		{"certificate", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: sec1})},
		{"non-P256 private", pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: wrongPrivate})},
		{"non-P256 public", pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: wrongPublic})},
		{"other public key", pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: otherPublic})},
		{"oversized", make([]byte, token.MaxKeySize+1)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := token.Encode([]byte(`{}`), "ES256", token.Key{PEM: tt.pem}); err == nil {
				t.Fatal("encode accepted invalid key")
			}
			got, err := token.Decode(fixture(t, "rfc7515-es256.jwt"), "ES256", token.Key{PEM: tt.pem}, time.Unix(1300819379, 0))
			if err == nil || errors.Is(err, token.ErrValidation) || got != nil {
				t.Fatalf("result = %v, error = %v; want key input error", got, err)
			}
		})
	}
	if got, err := token.Decode(fixture(t, "rfc7515-es256.jwt"), "", token.Key{PEM: privatePEM}, time.Unix(1300819379, 0)); err == nil || got != nil {
		t.Fatal("decode accepted private key as public key")
	}
}

func TestErrorsDoNotRevealInputs(t *testing.T) {
	marker := "sensitive-input-marker"
	for _, tt := range []struct {
		name string
		run  func() error
	}{
		{"body", func() error {
			_, err := token.Encode([]byte(marker), "HS256", token.Key{Secret: []byte(marker)})
			return err
		}},
		{"token", func() error { _, err := token.Decode(marker, "", token.Key{}, time.Time{}); return err }},
		{"key", func() error {
			_, err := token.Encode([]byte(`{}`), "ES256", token.Key{PEM: []byte(marker)})
			return err
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.run(); err == nil || strings.Contains(err.Error(), marker) {
				t.Fatalf("missing or leaking error: %v", err)
			}
		})
	}
}

func compact(header, payload, signature string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(header)) + "." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + signature
}

func hmacToken(header, payload string, secret []byte) string {
	raw := compact(header, payload, "")
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(strings.TrimSuffix(raw, ".")))
	return raw + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func fixture(t *testing.T, name string) string {
	t.Helper()
	body, err := os.ReadFile("../testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(body))
}

func exampleECKey(t *testing.T) (*ecdsa.PrivateKey, []byte) {
	t.Helper()
	var jwk struct{ X, Y, D string }
	if err := json.Unmarshal([]byte(fixture(t, "rfc7515-ec-key.jwk")), &jwk); err != nil {
		t.Fatal(err)
	}
	decodeInt := func(value string) *big.Int { return new(big.Int).SetBytes(decodeSegment(t, value)) }
	key := &ecdsa.PrivateKey{PublicKey: ecdsa.PublicKey{Curve: elliptic.P256(), X: decodeInt(jwk.X), Y: decodeInt(jwk.Y)}, D: decodeInt(jwk.D)}
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return key, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
}

func decodeSegment(t *testing.T, segment string) []byte {
	t.Helper()
	body, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func boolPointer(value bool) *bool { return &value }

func assertValidation(t *testing.T, got *token.Result, signature, claims string, valid *bool, codes []string) {
	t.Helper()
	if got == nil {
		t.Fatal("missing decoded result")
	}
	if codes == nil {
		codes = []string{}
	}
	want := token.Validation{Signature: signature, Claims: claims, Valid: valid, Errors: codes}
	if !reflect.DeepEqual(got.Validation, want) {
		t.Fatalf("validation = %+v, want %+v", got.Validation, want)
	}
}
