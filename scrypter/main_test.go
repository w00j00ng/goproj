package main

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestVerify(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "scrypter")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	if output, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}
	const stored = "00112233445566778899aabbccddeeff:69a78a4e2f2c0b88e195ee2b95216789cd67200921e02827f6038ecf545b9e9b1e1a9d6d6fa8c28f22432cbfdec55fbfab5e8289e0529924060c6a8e504e32e7"
	const empty = "00112233445566778899aabbccddeeff:497fd99302c442f6627fee539466649b1d40a6c71680cb681c10927ac69fddbfdfaab89fc61bc62faaa91b5099235a7e64362d00e1134a19ce7d03507e9178fa"
	tests := []struct {
		name   string
		args   []string
		code   int
		stdout string
		stderr string
	}{
		{"unsupported combining sequence", []string{"verify", "--hash", stored, "-p", "a" + strings.Repeat("\u0301", 40)}, 2, "", "unsupported password normalization"},
		{"matching flags", []string{"verify", "--hash", stored, "--password", "example-password"}, 0, "Password is valid.\n", ""},
		{"different password", []string{"verify", "--hash", stored, "-p", "wrong-password"}, 1, "Password does not match.\n", ""},
		{"malformed hash", []string{"verify", "--hash", "invalid", "-p", "example-password"}, 2, "", "invalid scrypt hash"},
		{"explicit empty hash", []string{"verify", "--hash=", "-p", "example-password"}, 2, "", "invalid scrypt hash"},
		{"explicit empty password", []string{"verify", "--hash", empty, "--password="}, 0, "Password is valid.\n", ""},
		{"missing hash without terminal", []string{"verify", "-p", "example-password"}, 2, "Enter Hash: \n", "read hash"},
		{"missing password without terminal", []string{"verify", "--hash", stored}, 2, "Enter Password: \n", "read password"},
		{"unexpected argument", []string{"verify", "extra", "--hash", stored, "-p", "example-password"}, 2, "", "unknown command"},
		{"unknown flag", []string{"verify", "--unknown"}, 2, "", "unknown flag"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := exec.Command(binary, tt.args...)
			command.Stdin = strings.NewReader("")
			var stdout, stderr bytes.Buffer
			command.Stdout = &stdout
			command.Stderr = &stderr
			err := command.Run()
			code := 0
			if err != nil {
				if exit, ok := err.(*exec.ExitError); ok {
					code = exit.ExitCode()
				} else {
					t.Fatal(err)
				}
			}
			if code != tt.code || stdout.String() != tt.stdout || !strings.Contains(stderr.String(), tt.stderr) {
				t.Fatalf("exit=%d, stdout=%q, stderr=%q; want exit=%d, stdout=%q, stderr containing %q", code, stdout.String(), stderr.String(), tt.code, tt.stdout, tt.stderr)
			}
			if tt.stderr == "" && stderr.Len() != 0 {
				t.Fatalf("unexpected stderr: %q", stderr.String())
			}
		})
	}
}
