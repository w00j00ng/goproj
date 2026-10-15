package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestVerifyInputs(t *testing.T) {
	password := "example-password"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	stored := string(hash)
	tests := []struct {
		name   string
		args   []string
		inputs []string
		want   string
	}{
		{"both flags", []string{"--hash", stored, "-p", password}, nil, "Password is valid.\n"},
		{"prompt password", []string{"--hash", stored}, []string{password}, "Enter Password: \nPassword is valid.\n"},
		{"prompt hash", []string{"-p", password}, []string{stored}, "Enter Hash: \nPassword is valid.\n"},
		{"both prompts", nil, []string{stored, password}, "Enter Hash: \nEnter Password: \nPassword is valid.\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reads := 0
			command := newVerifyCommand(func() ([]byte, error) {
				if reads >= len(tt.inputs) {
					return nil, errors.New("unexpected prompt")
				}
				value := tt.inputs[reads]
				reads++
				return []byte(value), nil
			})
			var output bytes.Buffer
			command.SetOut(&output)
			command.SetArgs(append([]string{}, tt.args...))
			if err := command.Execute(); err != nil {
				t.Fatal(err)
			}
			if output.String() != tt.want || reads != len(tt.inputs) {
				t.Fatalf("output=%q, reads=%d; want output=%q, reads=%d", output.String(), reads, tt.want, len(tt.inputs))
			}
		})
	}
}

func TestVerifyReadError(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{"hash", nil, "read hash"},
		{"password", []string{"--hash", "provided"}, "read password"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			readError := errors.New("terminal unavailable")
			command := newVerifyCommand(func() ([]byte, error) { return nil, readError })
			var output bytes.Buffer
			command.SetOut(&output)
			command.SetErr(&output)
			command.SetArgs(append([]string{}, tt.args...))
			err := command.Execute()
			if !errors.Is(err, readError) || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error=%v; want %q wrapping terminal error", err, tt.want)
			}
			if strings.Contains(output.String(), "does not match") {
				t.Fatalf("read failure reported as password mismatch: %q", output.String())
			}
		})
	}
}
