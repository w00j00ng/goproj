//go:build !windows && (!cgo || (!linux && !darwin) || android || ios)

package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestClipboardUnavailable(t *testing.T) {
	command := newRootCommand(func() ([]byte, error) { return []byte("example-password"), nil }, writeClipboard)
	var stdout, stderr bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.SetArgs([]string{"--clipboard"})
	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), "clipboard is unavailable") {
		t.Fatalf("Execute()=%v; want explicit clipboard error", err)
	}
	if strings.Contains(stdout.String()+stderr.String(), "copied on the clipboard!") {
		t.Fatal("unsupported clipboard reported success")
	}
}
