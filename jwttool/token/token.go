package token

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/golang-jwt/jwt/v5"
)

const MaxInputSize = 1 << 20
const MaxKeySize = 64 << 10

var ErrValidation = errors.New("token validation failed")

type Key struct {
	Secret []byte
	PEM    []byte
}

type Validation struct {
	Signature string   `json:"signature"`
	Claims    string   `json:"claims"`
	Valid     *bool    `json:"valid"`
	Errors    []string `json:"errors"`
}

type Result struct {
	Header     map[string]any  `json:"header"`
	Payload    json.RawMessage `json:"payload"`
	Validation Validation      `json:"validation"`
}

func Encode(body []byte, algorithm string, key Key) (string, error) {
	implementation := findAlgorithm(algorithm)
	if implementation == nil {
		return "", unsupportedAlgorithm()
	}
	if len(body) == 0 || len(body) > MaxInputSize {
		return "", errors.New("body is empty or exceeds size limit")
	}
	claims, err := jsonObject(body)
	if err != nil {
		return "", err
	}
	if err := validateKey(key); err != nil {
		return "", err
	}
	material, err := parseKey(implementation, key, true)
	if err != nil {
		return "", err
	}
	encoded, err := jwt.NewWithClaims(implementation.SigningMethod(), jwt.MapClaims(claims)).SignedString(material)
	if err != nil {
		return "", errors.New("could not sign token")
	}
	return encoded, nil
}

func Decode(raw string, algorithm string, key Key, now time.Time) (*Result, error) {
	if len(raw) == 0 || len(raw) > MaxInputSize {
		return nil, errors.New("token is empty or exceeds size limit")
	}
	raw = strings.TrimSpace(raw)
	if algorithm != "" && findAlgorithm(algorithm) == nil {
		return nil, unsupportedAlgorithm()
	}
	if err := validateKey(key); err != nil {
		return nil, err
	}
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, errors.New("token must contain three compact segments")
	}
	segments := make([][]byte, 3)
	for i, part := range parts {
		for _, ch := range []byte(part) {
			if !(ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '-' || ch == '_') {
				return nil, errors.New("token segments must use unpadded base64url")
			}
		}
		decoded, err := base64.RawURLEncoding.Strict().DecodeString(part)
		if err != nil {
			return nil, errors.New("invalid base64url token segment")
		}
		segments[i] = decoded
	}
	header, err := jsonObject(segments[0])
	if err != nil {
		return nil, err
	}
	claims, err := jsonObject(segments[1])
	if err != nil {
		return nil, err
	}
	headerAlgorithm, ok := header["alg"].(string)
	if !ok || headerAlgorithm == "" {
		return nil, errors.New("header must contain a nonempty algorithm")
	}
	if _, exists := header["crit"]; exists {
		return nil, errors.New("critical header extensions are not supported")
	}
	if b64, exists := header["b64"]; exists && b64 != true {
		return nil, errors.New("unencoded payloads are not supported")
	}
	result := &Result{
		Header:  header,
		Payload: segments[1],
		Validation: Validation{
			Signature: "not_checked",
			Claims:    "not_checked",
			Errors:    []string{},
		},
	}
	if key.Secret == nil && key.PEM == nil {
		if algorithm != "" && algorithm != headerAlgorithm {
			return nil, errors.New("header algorithm does not match requested algorithm")
		}
		return result, nil
	}
	valid := false
	result.Validation.Valid = &valid
	result.Validation.Signature = "invalid"
	selected := algorithm
	if selected == "" {
		selected = headerAlgorithm
	}
	implementation := findAlgorithm(selected)
	if implementation == nil || algorithm == "" && implementation.UsesSecret() != (key.Secret != nil) {
		result.Validation.Errors = append(result.Validation.Errors, "algorithm_mismatch")
		return result, ErrValidation
	}
	material, err := parseKey(implementation, key, false)
	if err != nil {
		return nil, err
	}
	if headerAlgorithm != implementation.SigningMethod().Alg() {
		result.Validation.Errors = append(result.Validation.Errors, "algorithm_mismatch")
		return result, ErrValidation
	}
	err = implementation.SigningMethod().Verify(parts[0]+"."+parts[1], segments[2], material)
	if err != nil {
		result.Validation.Errors = append(result.Validation.Errors, "signature_invalid")
		return result, ErrValidation
	}
	result.Validation.Signature = "valid"
	result.Validation.Claims = "valid"
	result.Validation.Errors = validateDates(claims, now)
	if len(result.Validation.Errors) != 0 {
		result.Validation.Claims = "invalid"
		return result, ErrValidation
	}
	valid = true
	return result, nil
}

func jsonObject(body []byte) (map[string]any, error) {
	invalid := errors.New("input must be a UTF-8 JSON object with unique member names")
	if !utf8.Valid(body) {
		return nil, invalid
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return nil, invalid
	}
	object := make(map[string]any)
	for decoder.More() {
		member, err := decoder.Token()
		if err != nil {
			return nil, invalid
		}
		name, ok := member.(string)
		if !ok {
			return nil, invalid
		}
		if _, duplicate := object[name]; duplicate {
			return nil, invalid
		}
		var value any
		if err := decoder.Decode(&value); err != nil {
			return nil, invalid
		}
		object[name] = value
	}
	if _, err := decoder.Token(); err != nil {
		return nil, invalid
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, invalid
	}
	return object, nil
}

func validateDates(claims map[string]any, now time.Time) []string {
	codes := []string{}
	// Compare exact JSON numbers so fractional dates do not inherit JWT's global time precision.
	instant := new(big.Rat).SetInt64(now.Unix())
	instant.Add(instant, new(big.Rat).SetFrac64(int64(now.Nanosecond()), int64(time.Second)))
	for _, name := range []string{"exp", "nbf"} {
		value, exists := claims[name]
		if !exists {
			continue
		}
		number, ok := value.(json.Number)
		if !ok {
			codes = append(codes, "claim_type_invalid")
			continue
		}
		date, ok := new(big.Rat).SetString(string(number))
		if !ok {
			codes = append(codes, "claim_type_invalid")
			continue
		}
		comparison := date.Cmp(instant)
		if name == "exp" && comparison <= 0 {
			codes = append(codes, "token_expired")
		}
		if name == "nbf" && comparison > 0 {
			codes = append(codes, "token_not_yet_valid")
		}
	}
	return codes
}
