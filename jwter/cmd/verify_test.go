package cmd

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/w00j00ng/goproj/jwter/token"
)

func TestVerifyCommand(t *testing.T) {
	secret := strings.Repeat("test", 8)
	body := `{"iss":"issuer","sub":"alice","aud":["api","other"],"exp":4102444800,"nbf":1,"iat":1,"jti":"id"}`
	unsigned := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`)) + "." + base64.RawURLEncoding.EncodeToString([]byte(body))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(unsigned))
	raw := unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	t.Setenv("JWTER_VERIFY_SECRET", secret)
	file := filepath.Join(t.TempDir(), "token.txt")
	if err := os.WriteFile(file, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	base := []string{"--alg", "HS256", "--secret-env", "JWTER_VERIFY_SECRET"}
	for _, tt := range []struct {
		name, stdin, code string
		args              []string
	}{
		{"positional", "", "", append([]string{raw}, base...)},
		{"token flag", "", "", append([]string{"--token", raw}, base...)},
		{"file", "", "", append([]string{"--token-file", file}, base...)},
		{"stdin", raw, "", append([]string{"--token-file", "-"}, base...)},
		{"all expected fields", "", "", append(append([]string{raw}, base...), "--iss", "issuer", "--sub", "alice", "--aud", "api", "--jti", "id", "--typ", "application/jwt", "--require", "exp,nbf,iat")},
		{"issuer mismatch", "", "iss_mismatch", append(append([]string{raw}, base...), "--iss", "other")},
		{"max age", "", "max_age_exceeded", append(append([]string{raw}, base...), "--max-age", "1h")},
		{"max lifetime", "", "max_lifetime_exceeded", append(append([]string{raw}, base...), "--max-lifetime", "1h")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			command := newRootCommand()
			command.SetArgs(append([]string{"verify"}, tt.args...))
			command.SetIn(strings.NewReader(tt.stdin))
			var out, stderr bytes.Buffer
			command.SetOut(&out)
			command.SetErr(&stderr)
			err := command.Execute()
			if tt.code == "" && err != nil || tt.code != "" && !errors.Is(err, token.ErrValidation) {
				t.Fatalf("error=%v; expected validation code %q", err, tt.code)
			}
			var result token.Result
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatalf("invalid result: %v", err)
			}
			if result.Header["alg"] != "HS256" || result.Header["typ"] != "JWT" || string(result.Payload) == "" || result.Validation.Valid == nil || *result.Validation.Valid != (tt.code == "") {
				t.Fatalf("unexpected verify result: %s", out.String())
			}
			if tt.code != "" && !strings.Contains(out.String(), tt.code) || stderr.Len() != 0 {
				t.Fatalf("unexpected output: stdout=%s stderr=%s", out.String(), stderr.String())
			}
		})
	}
}

func TestVerifyInputErrors(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{"missing key", []string{"sensitive-token", "--alg", "HS256"}, "verification key is required"},
		{"missing algorithm", []string{"sensitive-token", "--secret", "sensitive-secret"}, "--alg is required"},
		{"duplicate input", []string{"sensitive-token", "--token", "sensitive-token", "--alg", "HS256", "--secret", "sensitive-secret"}, "provide exactly one token input"},
		{"unknown required field", []string{"sensitive-token", "--alg", "HS256", "--secret", "sensitive-secret", "--require", "sensitive-field"}, "required fields must be"},
		{"empty required fields", []string{"sensitive-token", "--alg", "HS256", "--secret", "sensitive-secret", "--require="}, "required fields must not be empty"},
		{"empty expected value", []string{"sensitive-token", "--alg", "HS256", "--secret", "sensitive-secret", "--iss="}, "expected claim values must not be empty"},
		{"invalid expected media parameter", []string{"sensitive-token", "--alg", "HS256", "--secret", "sensitive-secret", "--typ", "JWT; profile*=sensitive-value"}, "expected typ must be a valid media type"},
		{"zero max age", []string{"sensitive-token", "--alg", "HS256", "--secret", "sensitive-secret", "--max-age", "0s"}, "positive durations"},
		{"negative max lifetime", []string{"sensitive-token", "--alg", "HS256", "--secret", "sensitive-secret", "--max-lifetime", "-1s"}, "positive durations"},
		{"negative leeway", []string{"sensitive-token", "--alg", "HS256", "--secret", "sensitive-secret", "--leeway", "-1s"}, "durations must not be negative"},
		{"invalid duration", []string{"sensitive-token", "--alg", "HS256", "--secret", "sensitive-secret", "--max-age", "sensitive-duration"}, "invalid flags"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			command := newRootCommand()
			command.SetArgs(append([]string{"verify"}, tt.args...))
			var output bytes.Buffer
			command.SetOut(&output)
			command.SetErr(&output)
			err := command.Execute()
			if err == nil || errors.Is(err, token.ErrValidation) || output.Len() != 0 || strings.Contains(err.Error(), "sensitive-") || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected safe input error, got error=%v output=%s", err, output.String())
			}
		})
	}
}
