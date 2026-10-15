package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/w00j00ng/goproj/scrypter/password"
)

func TestVerifyInputs(t *testing.T) {
	plain := "example-password"
	stored, err := password.Hash(plain)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		args   []string
		inputs []string
		want   string
	}{
		{"both flags", []string{"verify", "--hash", stored, "-p", plain}, nil, "Password is valid.\n"},
		{"prompt password", []string{"verify", "--hash", stored}, []string{plain}, "Enter Password: \nPassword is valid.\n"},
		{"prompt hash", []string{"verify", "-p", plain}, []string{stored}, "Enter Hash: \nPassword is valid.\n"},
		{"both prompts", []string{"verify"}, []string{stored, plain}, "Enter Hash: \nEnter Password: \nPassword is valid.\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reads := 0
			command := newRootCommand(func() ([]byte, error) {
				if reads >= len(tt.inputs) {
					return nil, errors.New("unexpected prompt")
				}
				value := tt.inputs[reads]
				reads++
				return []byte(value), nil
			})
			var output bytes.Buffer
			command.SetOut(&output)
			command.SetArgs(tt.args)
			if err := command.Execute(); err != nil {
				t.Fatal(err)
			}
			if output.String() != tt.want || reads != len(tt.inputs) {
				t.Fatalf("output=%q, reads=%d; want output=%q, reads=%d", output.String(), reads, tt.want, len(tt.inputs))
			}
		})
	}
}

func TestHashCommand(t *testing.T) {
	plain := "example-password"
	command := newRootCommand(func() ([]byte, error) { return []byte(plain), nil })
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetArgs([]string{})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(output.String(), "\n"), "\n")
	if len(lines) != 2 || lines[0] != "Enter Password: " {
		t.Fatalf("unexpected generation output: %q", output.String())
	}
	match, err := password.Verify(lines[1], plain)
	if err != nil || !match {
		t.Fatalf("generated CLI hash did not verify: match=%v, error=%v", match, err)
	}
}

func TestReadError(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{"generate", []string{}, "read password"},
		{"verify hash", []string{"verify"}, "read hash"},
		{"verify password", []string{"verify", "--hash", "provided"}, "read password"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			readError := errors.New("terminal unavailable")
			command := newRootCommand(func() ([]byte, error) { return nil, readError })
			var output bytes.Buffer
			command.SetOut(&output)
			command.SetErr(&output)
			command.SetArgs(tt.args)
			err := command.Execute()
			if !errors.Is(err, readError) || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error=%v; want %q wrapping terminal error", err, tt.want)
			}
			if strings.Contains(output.String(), "does not match") {
				t.Fatalf("read failure reported as mismatch: %q", output.String())
			}
		})
	}
}
