package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCLI(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "jwttool")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	if output, err := exec.Command("go", "build", "-mod=readonly", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}
	const secret = "cli-test-secret"
	sign := func(body, signingSecret string) string {
		unsigned := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`)) + "." + base64.RawURLEncoding.EncodeToString([]byte(body))
		mac := hmac.New(sha256.New, []byte(signingSecret))
		mac.Write([]byte(unsigned))
		return unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	}
	raw := sign(`{"sub":"user-123"}`, secret)
	verifySecret := strings.Repeat("test", 8)
	verifyRaw := sign(`{"iss":"issuer","sub":"user-123","aud":"api","iat":1,"exp":4102444800}`, verifySecret)
	t.Setenv("JWTTOOL_PROCESS_VERIFY_SECRET", verifySecret)
	for _, tt := range []struct {
		name   string
		args   []string
		stdin  string
		code   int
		stdout string
		stderr string
	}{
		{"help", nil, "", 0, "decode", ""},
		{"command help", []string{"help", "decode"}, "", 0, "--token", ""},
		{"unknown help topic", []string{"help", "sensitive-bad-token"}, "", 2, "", "unknown help topic"},
		{"positional token", []string{"decode", raw}, "", 0, `"valid": null`, ""},
		{"token flag", []string{"decode", "--token", raw}, "", 0, `"valid": null`, ""},
		{"verified token", []string{"decode", raw, "--secret", secret}, "", 0, `"valid": true`, ""},
		{"wrong secret", []string{"decode", "--token", raw, "--secret", "wrong-secret"}, "", 1, `"signature_invalid"`, ""},
		{"expired token", []string{"decode", "--token", sign(`{"exp":1}`, secret), "--secret", secret}, "", 1, `"token_expired"`, ""},
		{"verify success", []string{"verify", verifyRaw, "--alg", "HS256", "--secret-env", "JWTTOOL_PROCESS_VERIFY_SECRET", "--iss", "issuer", "--aud", "api", "--typ", "JWT", "--require", "exp"}, "", 0, `"valid": true`, ""},
		{"verify policy failure", []string{"verify", verifyRaw, "--alg", "HS256", "--secret-env", "JWTTOOL_PROCESS_VERIFY_SECRET", "--iss", "other"}, "", 1, `"iss_mismatch"`, ""},
		{"verify weak key", []string{"verify", raw, "--alg", "HS256", "--secret", secret}, "", 1, `"hmac_key_too_short"`, ""},
		{"verify age failure", []string{"verify", verifyRaw, "--alg", "HS256", "--secret-env", "JWTTOOL_PROCESS_VERIFY_SECRET", "--max-age", "1h"}, "", 1, `"max_age_exceeded"`, ""},
		{"verify lifetime failure", []string{"verify", verifyRaw, "--alg", "HS256", "--secret-env", "JWTTOOL_PROCESS_VERIFY_SECRET", "--max-lifetime", "1h"}, "", 1, `"max_lifetime_exceeded"`, ""},
		{"verify missing key", []string{"verify", verifyRaw, "--alg", "HS256"}, "", 2, "", "verification key is required"},
		{"verify missing algorithm", []string{"verify", verifyRaw, "--secret-env", "JWTTOOL_PROCESS_VERIFY_SECRET"}, "", 2, "", "--alg is required"},
		{"removed details flag", []string{"decode", raw, "--details"}, "", 2, "", "invalid flags"},
		{"wrong algorithm", []string{"decode", "--token", raw, "--alg", "ES256", "--secret", secret}, "", 2, "", ""},
		{"malformed", []string{"decode", "--token", "sensitive-bad-token", "--secret", secret}, "", 2, "", ""},
		{"missing token", []string{"decode"}, "", 2, "", "provide exactly one"},
		{"malformed positional token", []string{"decode", "sensitive-bad-token"}, "", 2, "", "three compact segments"},
		{"positional and flag", []string{"decode", raw, "--token", raw}, "", 2, "", "provide exactly one token input"},
		{"positional and file", []string{"decode", raw, "--token-file", "-"}, "", 2, "", "provide exactly one token input"},
		{"both flags", []string{"decode", "--token", raw, "--token-file", "-"}, "", 2, "", "provide exactly one token input"},
		{"multiple positional tokens", []string{"decode", "sensitive-bad-token", "extra"}, "", 2, "", "provide exactly one token input"},
		{"unexpected root argument", []string{"sensitive-bad-token"}, "", 2, "", "unexpected positional arguments"},
		{"stdin", []string{"decode", "--token-file", "-"}, raw + "\n", 0, `"sub": "user-123"`, ""},
		{"encode", []string{"encode", "--body", `{"sub":"user-123"}`, "--alg", "HS256", "--secret", secret}, "", 0, raw, ""},
		{"missing signing key", []string{"encode", "--body", "{}", "--alg", "HS256"}, "", 2, "", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			command := exec.Command(binary, tt.args...)
			command.Stdin = strings.NewReader(tt.stdin)
			var stdout, stderr bytes.Buffer
			command.Stdout, command.Stderr = &stdout, &stderr
			err := command.Run()
			code := 0
			if err != nil {
				if exit, ok := err.(*exec.ExitError); ok {
					code = exit.ExitCode()
				} else {
					t.Fatal(err)
				}
			}
			if code != tt.code || !strings.Contains(stdout.String(), tt.stdout) || !strings.Contains(stderr.String(), tt.stderr) {
				t.Fatalf("exit=%d stdout=%q stderr=%q; want exit=%d stdout containing %q stderr containing %q", code, stdout.String(), stderr.String(), tt.code, tt.stdout, tt.stderr)
			}
			if tt.code == 2 && stdout.Len() != 0 || tt.code != 2 && stderr.Len() != 0 {
				t.Fatalf("unexpected output stream: stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			if strings.Contains(stderr.String(), secret) || strings.Contains(stderr.String(), "sensitive-bad-token") {
				t.Fatalf("error exposed sensitive input: %q", stderr.String())
			}
			if len(tt.args) > 0 && (tt.args[0] == "decode" || tt.args[0] == "verify") && tt.code != 2 {
				var result struct {
					Header     map[string]any `json:"header"`
					Payload    map[string]any `json:"payload"`
					Validation map[string]any `json:"validation"`
				}
				if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if result.Header["alg"] != "HS256" || result.Header["typ"] != "JWT" || result.Payload == nil || result.Validation == nil {
					t.Fatalf("decode must include header, payload and validation: %s", stdout.String())
				}
				if tt.args[0] == "verify" && result.Validation["valid"] != (tt.code == 0) {
					t.Fatalf("verify validity does not match exit code: %s", stdout.String())
				}
			}
		})
	}

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	privateFile, publicFile := filepath.Join(dir, "private.pem"), filepath.Join(dir, "public.pem")
	for path, block := range map[string]*pem.Block{
		privateFile: {Type: "PRIVATE KEY", Bytes: privateDER},
		publicFile:  {Type: "PUBLIC KEY", Bytes: publicDER},
	} {
		if err := os.WriteFile(path, pem.EncodeToMemory(block), 0600); err != nil {
			t.Fatal(err)
		}
	}
	encoded, err := exec.Command(binary, "encode", "--body", `{"sub":"es256-user"}`, "--alg", "ES256", "--key-file", privateFile).CombinedOutput()
	if err != nil {
		t.Fatalf("encode ES256: %v: %s", err, encoded)
	}
	for _, tt := range []struct {
		name     string
		args     []string
		verified bool
	}{
		{"ES256 without key", []string{"decode", strings.TrimSpace(string(encoded))}, false},
		{"ES256 verified token", []string{"decode", strings.TrimSpace(string(encoded)), "--key-file", publicFile}, true},
		{"ES256 verify policy", []string{"verify", strings.TrimSpace(string(encoded)), "--alg", "ES256", "--key-file", publicFile, "--sub", "es256-user", "--typ", "JWT"}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			decoded, err := exec.Command(binary, tt.args...).CombinedOutput()
			if err != nil {
				t.Fatalf("decode ES256: %v: %s", err, decoded)
			}
			var result struct {
				Header     map[string]any  `json:"header"`
				Payload    json.RawMessage `json:"payload"`
				Validation *struct {
					Valid *bool `json:"valid"`
				} `json:"validation"`
			}
			if err := json.Unmarshal(decoded, &result); err != nil {
				t.Fatal(err)
			}
			if result.Header["alg"] != "ES256" || result.Header["typ"] != "JWT" || result.Validation == nil {
				t.Fatalf("decode must include ES256 JWT header and validation: %s", decoded)
			}
			var compact bytes.Buffer
			if err := json.Compact(&compact, result.Payload); err != nil || compact.String() != `{"sub":"es256-user"}` {
				t.Fatalf("payload=%s; want original ES256 body", result.Payload)
			}
			if (result.Validation.Valid != nil) != tt.verified || tt.verified && !*result.Validation.Valid {
				t.Fatalf("valid=%v; verified=%v", result.Validation.Valid, tt.verified)
			}
		})
	}
}
