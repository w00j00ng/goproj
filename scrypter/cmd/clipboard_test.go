package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/w00j00ng/goproj/scrypter/password"
)

func TestCopyHash(t *testing.T) {
	for _, tt := range []struct {
		name    string
		copyErr error
	}{
		{"success", nil},
		{"unavailable", errors.New("clipboard unavailable")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var copied []byte
			command := newRootCommand(func() ([]byte, error) { return []byte("example-password"), nil },
				func(hash []byte) error {
					if tt.copyErr != nil {
						return tt.copyErr
					}
					copied = hash
					return nil
				})
			var output bytes.Buffer
			command.SetOut(&output)
			command.SetErr(&output)
			command.SetArgs([]string{"--clipboard"})
			err := command.Execute()
			if tt.copyErr != nil {
				if !errors.Is(err, tt.copyErr) || strings.Contains(output.String(), "copied on the clipboard!") {
					t.Fatalf("err=%v output=%q", err, output.String())
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			match, err := password.Verify(string(copied), "example-password")
			if !match || err != nil {
				t.Fatalf("clipboard hash match=%v error=%v", match, err)
			}
			if !strings.Contains(output.String(), "copied on the clipboard!") || strings.Contains(output.String(), string(copied)) {
				t.Fatalf("unexpected clipboard output: %q", output.String())
			}
		})
	}
}
