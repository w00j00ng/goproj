package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestGenerate(t *testing.T) {
	readError := errors.New("terminal unavailable")
	copyError := errors.New("clipboard unavailable")
	for _, tt := range []struct {
		name    string
		plain   string
		args    []string
		readErr error
		copyErr error
		wantErr string
	}{
		{"password", "example-password", nil, nil, nil, ""},
		{"empty", "", nil, nil, nil, ""},
		{"spaces", " password ", nil, nil, nil, ""},
		{"72 bytes", strings.Repeat("a", 72), nil, nil, nil, ""},
		{"73 bytes", strings.Repeat("a", 73), nil, nil, nil, "password length exceeds 72 bytes"},
		{"read error", "", nil, readError, nil, "read password"},
		{"clipboard", "example-password", []string{"--clipboard"}, nil, nil, ""},
		{"clipboard error", "example-password", []string{"--clipboard"}, nil, copyError, "clipboard"},
		{"extra arg", "", []string{"extra"}, nil, nil, "unknown command"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var copied []byte
			command := newRootCommand(func() ([]byte, error) { return []byte(tt.plain), tt.readErr },
				func(hash []byte) error {
					if tt.copyErr != nil {
						return tt.copyErr
					}
					copied = hash
					return nil
				})
			var stdout, stderr bytes.Buffer
			command.SetOut(&stdout)
			command.SetErr(&stderr)
			command.SetArgs(append([]string{}, tt.args...))
			err := command.Execute()
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) || stdout.Len() != 0 || len(copied) != 0 {
					t.Fatalf("err=%v stdout=%q clipboard=%q", err, stdout.String(), copied)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			hash := []byte(strings.TrimSuffix(stdout.String(), "\n"))
			if len(tt.args) > 0 {
				hash = copied
				if stdout.Len() != 0 || !strings.Contains(stderr.String(), "copied on the clipboard!") {
					t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
				}
			} else if !strings.HasSuffix(stdout.String(), "\n") || strings.Count(stdout.String(), "\n") != 1 {
				t.Fatalf("hash must be one stdout line: %q", stdout.String())
			}
			if err := bcrypt.CompareHashAndPassword(hash, []byte(tt.plain)); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(stderr.String(), "Enter Password: ") {
				t.Fatalf("missing prompt on stderr: %q", stderr.String())
			}
		})
	}
}
