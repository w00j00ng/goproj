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
)

func TestCommands(t *testing.T) {
	const secret = "test-secret-for-cli"
	const body = `{"number":9007199254740993,"sub":"user-123"}`
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	unsigned := header + "." + base64.RawURLEncoding.EncodeToString([]byte(body))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(unsigned))
	raw := unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	t.Setenv("JWTTOOL_TEST_SECRET", secret)
	dir := t.TempDir()
	tokenFile := filepath.Join(dir, "token.txt")
	bodyFile := filepath.Join(dir, "body.json")
	for name, content := range map[string]string{tokenFile: raw, bodyFile: body} {
		if err := os.WriteFile(name, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, tt := range []struct {
		name    string
		args    []string
		stdin   string
		valid   *bool
		encode  bool
		details bool
	}{
		{name: "positional token", args: []string{"decode", raw}},
		{name: "positional details", args: []string{"decode", raw, "--details"}, details: true},
		{name: "details before positional token", args: []string{"decode", "--details", raw}, details: true},
		{name: "explicit body output", args: []string{"decode", raw, "--details=false"}},
		{name: "positional verified body", args: []string{"decode", raw, "--secret", secret}},
		{name: "inspect token flag", args: []string{"decode", "--token", raw}},
		{name: "token flag details", args: []string{"decode", "--token", raw, "--details"}, details: true},
		{name: "verify secret", args: []string{"decode", "--token", raw, "--secret", secret}, valid: boolPointer(true)},
		{name: "verify environment details", args: []string{"decode", raw, "--alg", "HS256", "--secret-env", "JWTTOOL_TEST_SECRET", "--details"}, valid: boolPointer(true), details: true},
		{name: "token file", args: []string{"decode", "--token-file", tokenFile}},
		{name: "token file details", args: []string{"decode", "--token-file", tokenFile, "--details"}, details: true},
		{name: "token stdin", args: []string{"decode", "--token-file", "-"}, stdin: "\n" + raw + "\n"},
		{name: "token stdin details", args: []string{"decode", "--token-file", "-", "--details"}, stdin: raw, details: true},
		{name: "encode body", args: []string{"encode", "--body", body, "--alg", "HS256", "--secret", secret}, encode: true},
		{name: "encode file", args: []string{"encode", "--body-file", bodyFile, "--alg", "HS256", "--secret-env", "JWTTOOL_TEST_SECRET"}, encode: true},
		{name: "encode stdin", args: []string{"encode", "--body-file", "-", "--alg", "HS256", "--secret", secret}, stdin: body, encode: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			command := newRootCommand()
			var stdout, stderr bytes.Buffer
			command.SetOut(&stdout)
			command.SetErr(&stderr)
			command.SetIn(strings.NewReader(tt.stdin))
			command.SetArgs(tt.args)
			if err := command.Execute(); err != nil {
				t.Fatal(err)
			}
			if stderr.Len() != 0 {
				t.Fatalf("unexpected stderr: %s", stderr.String())
			}
			if tt.encode {
				if stdout.String() != raw+"\n" {
					t.Fatalf("encoded token=%q; want %q", stdout.String(), raw+"\n")
				}
				return
			}
			if !tt.details {
				var compact bytes.Buffer
				if err := json.Compact(&compact, stdout.Bytes()); err != nil || compact.String() != body {
					t.Fatalf("output=%q; want body only %s", stdout.String(), body)
				}
				return
			}
			var result struct {
				Header     map[string]any  `json:"header"`
				Payload    json.RawMessage `json:"payload"`
				Validation struct {
					Valid *bool `json:"valid"`
				} `json:"validation"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
				t.Fatalf("invalid JSON output: %v: %s", err, stdout.String())
			}
			if result.Header["alg"] != "HS256" {
				t.Fatalf("header=%v; want HS256 header", result.Header)
			}
			var compact bytes.Buffer
			if err := json.Compact(&compact, result.Payload); err != nil || compact.String() != body {
				t.Fatalf("payload=%s; want %s", result.Payload, body)
			}
			if (result.Validation.Valid == nil) != (tt.valid == nil) || tt.valid != nil && *result.Validation.Valid != *tt.valid {
				t.Fatalf("valid=%v; want %v", result.Validation.Valid, tt.valid)
			}
		})
	}
}

func boolPointer(value bool) *bool { return &value }

func TestInputErrors(t *testing.T) {
	t.Setenv("JWTTOOL_TEST_EMPTY", "")
	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{"missing token", []string{"decode"}, "provide exactly one token input"},
		{"both token flags", []string{"decode", "--token", "sensitive-token", "--token-file", "-"}, "provide exactly one token input"},
		{"positional and token flag", []string{"decode", "sensitive-token", "--token", "sensitive-token"}, "provide exactly one token input"},
		{"positional and token file", []string{"decode", "sensitive-token", "--token-file", "-"}, "provide exactly one token input"},
		{"all token inputs", []string{"decode", "sensitive-token", "--token", "sensitive-token", "--token-file", "-"}, "provide exactly one token input"},
		{"missing body", []string{"encode", "--alg", "HS256", "--secret", "sensitive-secret"}, "provide exactly one of --body or --body-file"},
		{"both body inputs", []string{"encode", "--body", "{}", "--body-file", "-"}, "provide exactly one"},
		{"missing algorithm", []string{"encode", "--body", "{}", "--secret", "sensitive-secret"}, "--alg is required"},
		{"empty secret", []string{"decode", "--token", "token", "--secret="}, "secret must not be empty"},
		{"unset secret environment", []string{"decode", "--token", "token", "--secret-env", "JWTTOOL_TEST_EMPTY"}, "secret environment variable must be set and nonempty"},
		{"both secret inputs", []string{"decode", "--token", "token", "--secret", "sensitive-secret", "--secret-env", "JWTTOOL_TEST_EMPTY"}, "choose only one key input"},
		{"key and secret", []string{"decode", "--token", "token", "--secret", "sensitive-secret", "--key-file", "missing.pem"}, "choose only one key input"},
		{"missing file", []string{"decode", "--token-file", "does-not-exist.txt"}, "read token file"},
		{"unknown flag", []string{"decode", "--unknown"}, "invalid flags"},
		{"multiple positional tokens", []string{"decode", "sensitive-token", "sensitive-secret"}, "provide exactly one token input"},
		{"unexpected root argument", []string{"sensitive-token"}, "unexpected positional arguments"},
		{"unknown help topic", []string{"help", "sensitive-token"}, "unknown help topic"},
		{"invalid help value", []string{"decode", "--help=sensitive-secret"}, "invalid flags"},
		{"disabled algorithm", []string{"decode", "--token", "token", "--alg", "RS256"}, "algorithm must be one of: HS256, ES256"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			command := newRootCommand()
			command.SetArgs(tt.args)
			var output bytes.Buffer
			command.SetOut(&output)
			command.SetErr(&output)
			err := command.Execute()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error=%v; want %q", err, tt.want)
			}
			if strings.Contains(err.Error(), "sensitive-") || output.Len() != 0 {
				t.Fatalf("input error leaked values or wrote output: error=%v, output=%q", err, output.String())
			}
		})
	}
}

func TestAlgorithmHelp(t *testing.T) {
	for _, args := range [][]string{nil, {"help", "decode"}, {"help", "encode"}} {
		command := newRootCommand()
		command.SetArgs(args)
		var output bytes.Buffer
		command.SetOut(&output)
		if err := command.Execute(); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(output.String(), "HS256, ES256") {
			t.Fatalf("help does not show enabled algorithms: %s", output.String())
		}
	}
}

func TestInputLimitsAndIOErrors(t *testing.T) {
	const unsigned = "eyJhbGciOiJub25lIn0.e30."
	for _, tt := range []struct {
		name       string
		args       []string
		stdin      string
		readError  bool
		writeError bool
		want       string
	}{
		{name: "oversized token flag", args: []string{"decode", "--token", strings.Repeat("x", 1<<20+1)}, want: "exceeds"},
		{name: "oversized positional token", args: []string{"decode", strings.Repeat("x", 1<<20+1)}, want: "exceeds"},
		{name: "oversized token stdin", args: []string{"decode", "--token-file", "-"}, stdin: strings.Repeat("x", 1<<20+1), want: "exceeds"},
		{name: "stdin failure", args: []string{"decode", "--token-file", "-"}, readError: true, want: "read token"},
		{name: "output failure", args: []string{"decode", "--token", unsigned}, writeError: true, want: "test I/O failure"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			command := newRootCommand()
			command.SetArgs(tt.args)
			command.SetIn(strings.NewReader(tt.stdin))
			var output bytes.Buffer
			command.SetOut(&output)
			command.SetErr(&output)
			if tt.readError {
				command.SetIn(brokenIO{})
			}
			if tt.writeError {
				command.SetOut(brokenIO{})
			}
			err := command.Execute()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error=%v; want %q", err, tt.want)
			}
		})
	}
}

type brokenIO struct{}

func (brokenIO) Read([]byte) (int, error)  { return 0, errors.New("test I/O failure") }
func (brokenIO) Write([]byte) (int, error) { return 0, errors.New("test I/O failure") }
