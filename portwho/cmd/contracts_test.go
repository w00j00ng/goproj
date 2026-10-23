package cmd_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/w00j00ng/goproj/portwho/cmd"
	"github.com/w00j00ng/goproj/portwho/internal/inspect"
)

func TestRuntimeContracts(t *testing.T) {
	for _, tt := range []struct{ goos, arch, backend, socketScope, networkScope, pidScope, method string }{
		{"windows", "amd64", "windows_iphelper", "kernel_tcp", "host", "os_reported", "reported_owner"},
		{"linux", "amd64", "linux_sock_diag", "kernel_tcp", "current_network_namespace", "procfs_visible", "fd_holder"},
		{"linux", "arm64", "linux_sock_diag", "kernel_tcp", "current_network_namespace", "procfs_visible", "fd_holder"},
		{"darwin", "amd64", "darwin_pcblist64", "bsd_tcp", "host", "libproc_visible", "fd_holder"},
		{"darwin", "arm64", "darwin_pcblist64", "bsd_tcp", "host", "libproc_visible", "fd_holder"},
	} {
		t.Run(tt.goos+"/"+tt.arch, func(t *testing.T) {
			f := &fixture{runtime: inspect.Runtime{OS: tt.goos, Arch: tt.arch, Backend: tt.backend, SocketScope: tt.socketScope, NetworkScope: tt.networkScope, PIDScope: tt.pidScope}, families: map[string]inspect.FamilyResult{"ipv4": {Sockets: []inspect.Socket{socket("a", "ipv4", "127.0.0.1", 8080)}}}, mapping: map[string][]uint32{"a": {99}}, imagePath: "/tmp/한글 app"}
			var out, errout bytes.Buffer
			if code := cmd.Run([]string{"8080", "--json"}, &out, &errout, f); code != 0 {
				t.Fatalf("code=%d stderr=%s", code, errout.String())
			}
			var result inspect.Result
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.Runtime != f.runtime || result.Ownership.Method != tt.method || len(result.Listeners) != 1 || result.Listeners[0].Owners[0].PID != 99 {
				t.Fatalf("result=%+v", result)
			}
		})
	}
}

func TestUnavailablePlatforms(t *testing.T) {
	for _, code := range []string{"unsupported_platform", "backend_unavailable"} {
		t.Run(code, func(t *testing.T) {
			d := &inspect.Diagnostic{Stage: "platform", Code: code, Message: "backend unavailable"}
			f := &fixture{runtime: inspect.Runtime{OS: "darwin", Arch: "arm64", Backend: "unsupported", SocketScope: "unavailable", NetworkScope: "unavailable", PIDScope: "unavailable"}, families: map[string]inspect.FamilyResult{"ipv4": {Error: d}, "ipv6": {Error: d}}}
			var out, errout bytes.Buffer
			if status := cmd.Run([]string{"8080", "--json"}, &out, &errout, f); status != 3 {
				t.Fatalf("code=%d", status)
			}
			var result inspect.Result
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.Collection.Complete || result.Collection.IPv4 != "failed" || result.Collection.IPv6 != "failed" || len(result.Errors) != 1 || len(result.Listeners) != 0 || result.Ownership.Method != "unavailable" {
				t.Fatalf("result=%+v", result)
			}
		})
	}
}

func TestTableDiagnosticsAndEscaping(t *testing.T) {
	for _, tt := range []struct {
		name               string
		imageError         error
		imagePath, want    string
		unresolved, failed bool
		code               int
	}{
		{name: "access denied once per PID", imageError: os.ErrPermission, want: "<access-denied>"},
		{name: "escape untrusted filename", imagePath: "/tmp/한글\napp\t\x1b\u202e", want: `한글\napp\t\u001b\u202e`},
		{name: "preserve unresolved table", unresolved: true, want: "unresolved", code: 3},
		{name: "partial collection table", failed: true, want: "resolved", code: 3},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := &fixture{families: map[string]inspect.FamilyResult{"ipv4": {Sockets: []inspect.Socket{socket("a", "ipv4", "127.0.0.1", 8080)}}, "ipv6": {Sockets: []inspect.Socket{socket("b", "ipv6", "::1", 8080)}}}, mapping: map[string][]uint32{"a": {99}, "b": {99}}, imageError: tt.imageError, imagePath: tt.imagePath}
			if tt.unresolved {
				f.mapping = nil
			}
			if tt.failed {
				f.families["ipv6"] = inspect.FamilyResult{Error: collectionFailure("ipv6", "collection_failed")}
			}
			var out, errout bytes.Buffer
			if code := cmd.Run([]string{"8080", "--path"}, &out, &errout, f); code != tt.code {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errout.String())
			}
			if !strings.Contains(out.String(), tt.want) || !strings.Contains(out.String(), "PATH") {
				t.Fatalf("stdout=%q", out.String())
			}
			if tt.imageError != nil && strings.Count(errout.String(), "PID 99") != 1 {
				t.Fatalf("warnings=%q", errout.String())
			}
			if strings.Contains(out.String(), "\x1b") || strings.Contains(out.String(), "\u202e") {
				t.Fatal("control/format character leaked")
			}
			if tt.unresolved && !strings.Contains(out.String(), "?") {
				t.Fatal("missing unknown-owner marker")
			}
		})
	}
}

func TestAddressOrderingAndInvalidUTF8(t *testing.T) {
	f := &fixture{families: map[string]inspect.FamilyResult{"ipv4": {Sockets: []inspect.Socket{socket("10", "ipv4", "127.0.0.10", 8080), socket("2", "ipv4", "127.0.0.2", 8080)}}}, mapping: map[string][]uint32{"10": {99}, "2": {99}}, imagePath: "/tmp/bad\xffname"}
	var out, errout bytes.Buffer
	if code := cmd.Run([]string{"8080", "--json", "--path"}, &out, &errout, f); code != 0 {
		t.Fatalf("code=%d", code)
	}
	var result inspect.Result
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	addresses := []string{}
	for _, l := range result.Listeners {
		addresses = append(addresses, l.Local.Address)
		if l.Owners[0].Process.Name == nil || *l.Owners[0].Process.Name != "bad\uFFFDname" {
			t.Fatal(l.Owners[0].Process)
		}
	}
	if !reflect.DeepEqual(addresses, []string{"127.0.0.2", "127.0.0.10"}) {
		t.Fatal(addresses)
	}
}

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }
func TestShortWritesAndStderrFailure(t *testing.T) {
	f := &fixture{families: map[string]inspect.FamilyResult{"ipv4": {Sockets: []inspect.Socket{socket("a", "ipv4", "127.0.0.1", 8080)}}}, mapping: map[string][]uint32{"a": {99}}, imageError: os.ErrPermission}
	for _, tt := range []struct {
		name        string
		out, errout io.Writer
	}{
		{"short stdout", shortWriter{}, io.Discard}, {"metadata warning stderr", io.Discard, &brokenWriter{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if code := cmd.Run([]string{"8080"}, tt.out, tt.errout, f); code != 3 {
				t.Fatalf("code=%d", code)
			}
		})
	}
}
