package cmd_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/netip"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/w00j00ng/goproj/portwho/cmd"
	"github.com/w00j00ng/goproj/portwho/internal/inspect"
)

type fixture struct {
	families    map[string]inspect.FamilyResult
	mapping     map[string][]uint32
	limitations []string
	imageError  error
	imagePath   string
	runtime     inspect.Runtime
	collected   []string
	resolved    []inspect.Socket
	images      []uint32
}

func (f *fixture) Runtime() inspect.Runtime {
	if f.runtime.OS != "" {
		return f.runtime
	}
	return inspect.Runtime{OS: "windows", Arch: "amd64", Backend: "windows_iphelper", SocketScope: "kernel_tcp", NetworkScope: "host", PIDScope: "os_reported"}
}
func (f *fixture) Collect(families []string) map[string]inspect.FamilyResult {
	f.collected = append(f.collected, families...)
	result := make(map[string]inspect.FamilyResult)
	for _, family := range families {
		result[family] = f.families[family]
	}
	return result
}
func (f *fixture) Resolve(sockets []inspect.Socket) (map[string][]uint32, []string) {
	f.resolved = append(f.resolved, sockets...)
	return f.mapping, f.limitations
}
func (f *fixture) ImagePath(pid uint32) (string, error) {
	f.images = append(f.images, pid)
	if f.imagePath != "" {
		return f.imagePath, f.imageError
	}
	return `C:\한글 앱\backend.exe`, f.imageError
}
func socket(id, family, addr string, port uint16) inspect.Socket {
	return inspect.Socket{ID: id, Family: family, Address: netip.MustParseAddr(addr), Port: port, State: "LISTEN"}
}
func collectionFailure(family, code string) *inspect.Diagnostic {
	return &inspect.Diagnostic{Stage: "collection", Family: &family, Code: code, Message: "fixture collection failed"}
}

func TestQueryContract(t *testing.T) {
	v4 := socket("a", "ipv4", "127.0.0.1", 8080)
	v6 := socket("b", "ipv6", "::1", 8080)
	other := socket("c", "ipv4", "127.0.0.1", 18080)
	established := socket("d", "ipv4", "127.0.0.1", 8080)
	established.State = "ESTABLISHED"
	scope1 := socket("s1", "ipv6", "fe80::1", 8080)
	scope1.ScopeID = 1
	scope2 := scope1
	scope2.ID = "s2"
	scope2.ScopeID = 2
	tests := []struct {
		name        string
		args        []string
		v4, v6      []inspect.Socket
		mapping     map[string][]uint32
		limitations []string
		failure     *inspect.Diagnostic
		imageError  error
		code        int
		statuses    []string
		pids        [][]uint32
	}{
		{name: "A01 filter listen and port", args: []string{"8080"}, v4: []inspect.Socket{v4, other, established}, mapping: map[string][]uint32{"a": {9}}, statuses: []string{"resolved"}, pids: [][]uint32{{9}}},
		{name: "A02 ports are OR", args: []string{"8080", "9090"}, v4: []inspect.Socket{v4}, mapping: map[string][]uint32{"a": {9}}, statuses: []string{"resolved"}, pids: [][]uint32{{9}}},
		{name: "A03 empty", args: []string{"8080"}, code: 1},
		{name: "A04 all", args: []string{"--all"}, v4: []inspect.Socket{other, established, v4}, mapping: map[string][]uint32{"a": {9}, "c": {8}}, statuses: []string{"resolved", "resolved"}, pids: [][]uint32{{9}, {8}}},
		{name: "A05 separate families", args: []string{"8080"}, v4: []inspect.Socket{v4}, v6: []inspect.Socket{v6}, mapping: map[string][]uint32{"a": {9}, "b": {9}}, statuses: []string{"resolved", "resolved"}, pids: [][]uint32{{9}, {9}}},
		{name: "A06 requested family", args: []string{"--family", "ipv4", "8080"}, v4: []inspect.Socket{v4}, v6: []inspect.Socket{v6}, mapping: map[string][]uint32{"a": {9}}, statuses: []string{"resolved"}, pids: [][]uint32{{9}}},
		{name: "A07 empty requested v6", args: []string{"8080", "--family=ipv6"}, v4: []inspect.Socket{v4}, code: 1},
		{name: "A20 metadata access denied", args: []string{"8080"}, v4: []inspect.Socket{v4}, mapping: map[string][]uint32{"a": {9}}, imageError: os.ErrPermission, statuses: []string{"resolved"}, pids: [][]uint32{{9}}},
		{name: "A21 A22 metadata OS error", args: []string{"8080"}, v4: []inspect.Socket{v4}, mapping: map[string][]uint32{"a": {9}}, imageError: errors.New("error 87"), statuses: []string{"resolved"}, pids: [][]uint32{{9}}},
		{name: "A24 preserve good family", args: []string{"8080"}, v4: []inspect.Socket{v4}, v6: []inspect.Socket{v6}, mapping: map[string][]uint32{"a": {9}, "b": {8}}, failure: collectionFailure("ipv6", "collection_failed"), code: 3, statuses: []string{"resolved"}, pids: [][]uint32{{9}}},
		{name: "A26 failed plus empty", args: []string{"8080"}, failure: collectionFailure("ipv6", "collection_failed"), code: 3},
		{name: "A27 no invented ipv4 wildcard", args: []string{"8080"}, v6: []inspect.Socket{socket("b", "ipv6", "::", 8080)}, mapping: map[string][]uint32{"b": {9}}, statuses: []string{"resolved"}, pids: [][]uint32{{9}}},
		{name: "A28 scopes differ", args: []string{"8080"}, v6: []inspect.Socket{scope2, scope1}, mapping: map[string][]uint32{"s1": {9}, "s2": {8}}, statuses: []string{"resolved", "resolved"}, pids: [][]uint32{{9}, {8}}},
		{name: "A33 malformed native data", args: []string{"8080"}, failure: collectionFailure("ipv6", "invalid_os_data"), code: 3},
		{name: "A34 resource limit", args: []string{"8080"}, failure: collectionFailure("ipv6", "resource_limit"), code: 3},
		{name: "A35 preserve unresolved", args: []string{"8080"}, v4: []inspect.Socket{v4}, code: 3, statuses: []string{"unresolved"}, pids: [][]uint32{{}}},
		{name: "A37 A38 shared socket duplicate fd", args: []string{"8080"}, v4: []inspect.Socket{v4}, mapping: map[string][]uint32{"a": {9, 2, 9}}, statuses: []string{"resolved"}, pids: [][]uint32{{2, 9}}},
		{name: "A39 reuseport union", args: []string{"8080"}, v4: []inspect.Socket{v4, socket("e", "ipv4", "127.0.0.1", 8080)}, mapping: map[string][]uint32{"a": {9}, "e": {2}}, statuses: []string{"resolved"}, pids: [][]uint32{{2, 9}}},
		{name: "A40 reuseport partial", args: []string{"8080"}, v4: []inspect.Socket{v4, socket("e", "ipv4", "127.0.0.1", 8080)}, mapping: map[string][]uint32{"a": {9}}, code: 3, statuses: []string{"partial"}, pids: [][]uint32{{9}}},
		{name: "A41 A42 visibility not completeness", args: []string{"8080"}, v4: []inspect.Socket{v4}, mapping: map[string][]uint32{"a": {9}}, limitations: []string{"restricted_procfs", "permission_denied", "permission_denied"}, statuses: []string{"resolved"}, pids: [][]uint32{{9}}},
		{name: "real PID zero", args: []string{"8080"}, v4: []inspect.Socket{v4}, mapping: map[string][]uint32{"a": {0}}, statuses: []string{"resolved"}, pids: [][]uint32{{0}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fixture{families: map[string]inspect.FamilyResult{"ipv4": {Sockets: tt.v4}, "ipv6": {Sockets: tt.v6, Error: tt.failure}}, mapping: tt.mapping, limitations: tt.limitations, imageError: tt.imageError}
			var out, errout bytes.Buffer
			args := append(append([]string{}, tt.args...), "--json")
			if code := cmd.Run(args, &out, &errout, f); code != tt.code {
				t.Fatalf("code=%d want %d; stderr=%s", code, tt.code, errout.String())
			}
			var got inspect.Result
			decoder := json.NewDecoder(&out)
			if err := decoder.Decode(&got); err != nil {
				t.Fatal(err)
			}
			if err := decoder.Decode(new(any)); err != io.EOF {
				t.Fatalf("not one JSON object: %v", err)
			}
			if got.SchemaVersion != 2 || got.Listeners == nil || got.Errors == nil || got.Ownership.Limitations == nil {
				t.Fatalf("missing required fields: %+v", got)
			}
			if tt.args[0] == "--all" && got.Query.Ports != nil {
				t.Fatal("--all must encode ports=null")
			}
			if got.Runtime.Backend != "windows_iphelper" || got.Ownership.Mode != "best_effort" || got.Ownership.Method != "reported_owner" {
				t.Fatalf("runtime/ownership: %+v", got)
			}
			for _, stamp := range []string{got.Collection.StartedAt, got.Collection.FinishedAt} {
				if _, e := time.Parse(time.RFC3339Nano, stamp); e != nil || !strings.HasSuffix(stamp, "Z") {
					t.Fatalf("timestamp %q", stamp)
				}
			}
			if got.Collection.Complete != (tt.failure == nil) {
				t.Fatalf("collection complete=%v", got.Collection.Complete)
			}
			if len(got.Listeners) != len(tt.statuses) {
				t.Fatalf("listeners=%+v", got.Listeners)
			}
			for i, l := range got.Listeners {
				if l.Protocol != "tcp" || l.State != "LISTEN" || l.OwnershipStatus != tt.statuses[i] || l.Owners == nil {
					t.Fatalf("listener=%+v", l)
				}
				pids := []uint32{}
				for _, o := range l.Owners {
					pids = append(pids, o.PID)
					if tt.imageError == nil {
						if o.Process.Name == nil || *o.Process.Name != "backend.exe" || o.Process.Path != nil || o.Process.Reason != nil || o.Process.Status != "ok" {
							t.Fatalf("process=%+v", o.Process)
						}
					} else {
						reason := "os_error"
						if errors.Is(tt.imageError, os.ErrPermission) {
							reason = "access_denied"
						}
						if o.Process.Status != "unavailable" || o.Process.Name != nil || o.Process.Path != nil || o.Process.Reason == nil || *o.Process.Reason != reason {
							t.Fatalf("failed process=%+v", o.Process)
						}
					}
				}
				if !reflect.DeepEqual(pids, tt.pids[i]) {
					t.Fatalf("pids=%v want %v", pids, tt.pids[i])
				}
			}
			if tt.code == 3 && (len(got.Errors) == 0 || errout.Len() == 0) {
				t.Fatal("core errors must appear in JSON and stderr")
			}
			if tt.code != 3 && (len(got.Errors) != 0 || errout.Len() != 0) {
				t.Fatalf("metadata/limitations leaked to stderr: %s", errout.String())
			}
			if len(tt.limitations) > 0 && !reflect.DeepEqual(got.Ownership.Limitations, []string{"permission_denied", "restricted_procfs"}) {
				t.Fatal(got.Ownership.Limitations)
			}
			if got.Query.Family == "ipv4" && (!reflect.DeepEqual(f.collected, []string{"ipv4"}) || got.Collection.IPv6 != "not_requested") {
				t.Fatal("unrequested ipv6 collected")
			}
			if len(tt.statuses) == 0 && len(f.resolved) != 0 {
				t.Fatal("unnecessary PID scan")
			}
			seen := map[uint32]bool{}
			for _, pid := range f.images {
				if seen[pid] {
					t.Fatal("metadata queried twice")
				}
				seen[pid] = true
			}
		})
	}
}

func TestUsageAndInformationalRequests(t *testing.T) {
	tests := []struct {
		args []string
		code int
	}{
		{[]string{}, 2}, {[]string{"--all", "8080"}, 2}, {[]string{"8080", "--json", "--pids"}, 2}, {[]string{"8080", "--path", "--pids"}, 2},
		{[]string{"8080", "--family", "IPv4"}, 2}, {[]string{"8080", "--family", "any"}, 2}, {[]string{"8080", "--family"}, 2}, {[]string{"8080", "--unknown"}, 2},
		{[]string{"--help"}, 0}, {[]string{"-h"}, 0}, {[]string{"--version"}, 0},
	}
	for _, port := range []string{"0", "65536", "-1", "+80", "0x50", "８０", "80.0", " 8080 ", "80,443", "8000-8100", "http", "localhost:8080", "99999999999999999999999"} {
		tests = append(tests, struct {
			args []string
			code int
		}{[]string{port}, 2})
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			var out, errout bytes.Buffer
			f := &fixture{}
			if code := cmd.Run(tt.args, &out, &errout, f); code != tt.code {
				t.Fatalf("code=%d want %d", code, tt.code)
			}
			if len(f.collected) > 0 || len(f.resolved) > 0 || len(f.images) > 0 {
				t.Fatal("OS queried")
			}
			if tt.code == 2 && (out.Len() != 0 || !strings.Contains(errout.String(), "Usage:")) {
				t.Fatalf("stdout=%q stderr=%q", out.String(), errout.String())
			}
			if tt.code == 0 && (out.Len() == 0 || errout.Len() != 0) {
				t.Fatalf("stdout=%q stderr=%q", out.String(), errout.String())
			}
			if len(tt.args) == 1 && tt.args[0] == "--version" && out.String() != "portwho "+cmd.Version+"\n" {
				t.Fatalf("version=%q", out.String())
			}
		})
	}
}

func TestOutputModes(t *testing.T) {
	tests := []struct {
		name               string
		args               []string
		want               string
		code               int
		unresolved, failed bool
		metadata           bool
	}{
		{"A08 A09 flags and normalization", []string{"--json", "08080", "5432", "8080", "--family", "both"}, "", 0, false, false, true},
		{"A10 A11 pid sorted unique", []string{"8080", "--pids"}, "2\n9\n", 0, false, false, false},
		{"A13 A29 unicode path", []string{"8080", "--path", "--json"}, "", 0, false, false, true},
		{"A14 no path table", []string{"8080"}, "", 0, false, false, true},
		{"A25 failed pids", []string{"8080", "--pids"}, "", 3, false, true, false},
		{"A36 unresolved pids", []string{"8080", "--pids"}, "", 3, true, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fixture{families: map[string]inspect.FamilyResult{"ipv4": {Sockets: []inspect.Socket{socket("a", "ipv4", "127.0.0.1", 8080)}}, "ipv6": {Sockets: []inspect.Socket{socket("b", "ipv6", "::1", 8080)}}}, mapping: map[string][]uint32{"a": {9, 2}, "b": {9}}}
			if tt.unresolved {
				delete(f.mapping, "a")
			}
			if tt.failed {
				f.families["ipv6"] = inspect.FamilyResult{Error: collectionFailure("ipv6", "collection_failed")}
			}
			var out, errout bytes.Buffer
			if code := cmd.Run(tt.args, &out, &errout, f); code != tt.code {
				t.Fatalf("code=%d stderr=%s", code, errout.String())
			}
			if !tt.metadata && len(f.images) != 0 {
				t.Fatal("PID mode read image metadata")
			}
			if strings.Contains(strings.Join(tt.args, " "), "--pids") && out.String() != tt.want {
				t.Fatalf("out=%q want=%q", out.String(), tt.want)
			}
			if strings.Contains(strings.Join(tt.args, " "), "--json") {
				var r inspect.Result
				if e := json.Unmarshal(out.Bytes(), &r); e != nil {
					t.Fatal(e)
				}
				if tt.name == "A08 A09 flags and normalization" && !reflect.DeepEqual(r.Query.Ports, []uint16{5432, 8080}) {
					t.Fatal(r.Query.Ports)
				}
				if r.Query.IncludePath && (r.Listeners[0].Owners[0].Process.Path == nil || *r.Listeners[0].Owners[0].Process.Path != `C:\한글 앱\backend.exe`) {
					t.Fatal("path not preserved")
				}
			} else if tt.metadata {
				if strings.Contains(out.String(), "PATH") || strings.Contains(out.String(), `C:\`) {
					t.Fatal("unexpected image path")
				}
				if !strings.Contains(out.String(), "[::1]:8080") {
					t.Fatal(out.String())
				}
			}
		})
	}
}

type brokenWriter struct{ calls int }

func (w *brokenWriter) Write(p []byte) (int, error) { w.calls++; return 0, io.ErrClosedPipe }
func TestWriterFailures(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"--version"}, {"8080", "--json"}, {"8080"}, {"8080", "--pids"}, {"bad"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			out, errout := &brokenWriter{}, &brokenWriter{}
			f := &fixture{families: map[string]inspect.FamilyResult{"ipv4": {Sockets: []inspect.Socket{socket("a", "ipv4", "127.0.0.1", 8080)}}}, mapping: map[string][]uint32{"a": {9}}}
			if code := cmd.Run(args, out, errout, f); code != 3 {
				t.Fatalf("code=%d", code)
			}
			if out.calls > 2 || errout.calls > 2 {
				t.Fatalf("repeated reporting: %d %d", out.calls, errout.calls)
			}
		})
	}
}

func TestEmptyAndPartialTable(t *testing.T) {
	for _, mode := range []string{"table", "json", "pids"} {
		t.Run(mode, func(t *testing.T) {
			for _, failed := range []bool{false, true} {
				f := &fixture{families: map[string]inspect.FamilyResult{}}
				if failed {
					f.families["ipv6"] = inspect.FamilyResult{Error: collectionFailure("ipv6", "collection_failed")}
				}
				args := []string{"8080"}
				if mode != "table" {
					args = append(args, "--"+mode)
				}
				var out, errout bytes.Buffer
				want := 1
				if failed {
					want = 3
				}
				if got := cmd.Run(args, &out, &errout, f); got != want {
					t.Fatalf("code=%d", got)
				}
				if mode == "table" {
					wantOut := "No matching TCP listeners found.\n"
					if failed {
						wantOut = ""
					}
					if out.String() != wantOut {
						t.Fatalf("out=%q", out.String())
					}
				}
				if mode == "pids" && out.Len() != 0 {
					t.Fatal("empty PID stdout required")
				}
			}
		})
	}
}
