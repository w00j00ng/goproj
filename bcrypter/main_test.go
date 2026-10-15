package main

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestVerify(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "bcrypter")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	if output, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}

	hash := func(password string) string {
		t.Helper()
		value, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
		if err != nil {
			t.Fatal(err)
		}
		return string(value)
	}
	password := "example-password"
	stored := hash(password)
	boundary := strings.Repeat("a", 72)
	boundaryHash := hash(boundary)

	tests := []struct {
		name string
		args []string
		code int
		want string
	}{
		{"matching flags", []string{"verify", "--hash", stored, "--password", password}, 0, "Password is valid.\n"},
		{"different password", []string{"verify", "--hash", stored, "-p", "wrong-password"}, 1, "Password does not match.\n"},
		{"malformed hash", []string{"verify", "--hash", "invalid", "-p", password}, 2, "invalid bcrypt hash"},
		{"invalid hash body", []string{"verify", "--hash", stored[:len(stored)-1] + "!", "-p", password}, 2, "invalid bcrypt hash"},
		{"trailing hash data", []string{"verify", "--hash", stored + "extra", "-p", password}, 2, "invalid bcrypt hash"},
		{"corrupted separator", []string{"verify", "--hash", strings.Replace(stored, "$04$", "!04$", 1), "-p", password}, 2, "invalid bcrypt hash"},
		{"explicit empty hash", []string{"verify", "--hash=", "-p", password}, 2, "invalid bcrypt hash"},
		{"explicit empty password", []string{"verify", "--hash", hash(""), "--password="}, 0, "Password is valid.\n"},
		{"preserve spaces", []string{"verify", "--hash", hash(" password "), "-p", " password "}, 0, "Password is valid.\n"},
		{"72 byte password", []string{"verify", "--hash", boundaryHash, "-p", boundary}, 0, "Password is valid.\n"},
		{"over 72 bytes", []string{"verify", "--hash", boundaryHash, "-p", boundary + "x"}, 2, "password length exceeds 72 bytes"},
		{"missing hash without terminal", []string{"verify", "-p", password}, 2, "read hash"},
		{"missing password without terminal", []string{"verify", "--hash", stored}, 2, "read password"},
		{"unexpected argument", []string{"verify", "extra", "--hash", stored, "-p", password}, 2, "unknown command"},
		{"unknown flag", []string{"verify", "--unknown"}, 2, "unknown flag"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := exec.Command(binary, tt.args...)
			command.Stdin = strings.NewReader("")
			var stdout, stderr bytes.Buffer
			command.Stdout = &stdout
			command.Stderr = &stderr
			err := command.Run()
			output := stdout.String() + stderr.String()
			code := 0
			if err != nil {
				if exit, ok := err.(*exec.ExitError); ok {
					code = exit.ExitCode()
				} else {
					t.Fatal(err)
				}
			}
			if code != tt.code || !strings.Contains(output, tt.want) {
				t.Fatalf("exit=%d, output=%q; want exit=%d, output containing %q", code, output, tt.code, tt.want)
			}
			if tt.code < 2 && (stdout.String() != tt.want || stderr.Len() != 0) {
				t.Fatalf("stdout=%q, stderr=%q; want result on stdout only", stdout.String(), stderr.String())
			}
		})
	}
}
