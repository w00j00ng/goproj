package platform

import (
	"encoding/binary"
	"net/netip"
	"reflect"
	"testing"

	"github.com/w00j00ng/goproj/portwho/internal/inspect"
)

func linuxMessage(kind, flags uint16, seq, pid uint32, body []byte) []byte {
	n := 16 + len(body)
	b := make([]byte, (n+3)&^3)
	binary.LittleEndian.PutUint32(b, uint32(n))
	binary.LittleEndian.PutUint16(b[4:], kind)
	binary.LittleEndian.PutUint16(b[6:], flags)
	binary.LittleEndian.PutUint32(b[8:], seq)
	binary.LittleEndian.PutUint32(b[12:], pid)
	copy(b[16:], body)
	return b
}

func linuxDiagBody(family byte, address netip.Addr, port uint16, inode uint32) []byte {
	b := make([]byte, 72)
	b[0], b[1] = family, 10
	binary.BigEndian.PutUint16(b[4:], port)
	copy(b[8:], address.AsSlice())
	binary.LittleEndian.PutUint32(b[40:], 7)
	binary.LittleEndian.PutUint32(b[44:], 123)
	binary.LittleEndian.PutUint32(b[48:], 456)
	binary.LittleEndian.PutUint32(b[64:], 1000)
	binary.LittleEndian.PutUint32(b[68:], inode)
	return b
}

func TestLinuxNetlinkSocketRecords(t *testing.T) {
	for _, tc := range []struct {
		name, family, address string
		native                byte
		scope                 uint32
	}{
		{"ipv4", "ipv4", "127.0.0.1", 2, 0},
		{"ipv6", "ipv6", "::1", 10, 0},
		{"scoped", "ipv6", "fe80::1234", 10, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := linuxDiagBody(tc.native, netip.MustParseAddr(tc.address), 8080, 789)
			input := linuxMessage(20, 2, 42, 999, body)
			input = append(input, linuxMessage(3, 2, 42, 999, make([]byte, 4))...)
			got, done, err := decodeLinuxNetlink(input, tc.family, 42, 999, 0)
			if err != nil || !done || len(got) != 1 {
				t.Fatalf("records=%v done=%v error=%v", got, done, err)
			}
			s := got[0]
			if s.Address.String() != tc.address || s.Port != 8080 || s.ScopeID != tc.scope || s.State != "LISTEN" || s.Inode != 789 || s.Cookie != [2]uint32{123, 456} || s.UID != 1000 || len(s.PIDs) != 0 {
				t.Fatalf("decoded socket: %+v", s)
			}
		})
	}
}

func TestLinuxNetlinkRejectsIncompleteOrForeignDump(t *testing.T) {
	valid := linuxMessage(20, 2, 42, 999, linuxDiagBody(2, netip.MustParseAddr("127.0.0.1"), 8080, 789))
	errBody := make([]byte, 20)
	binary.LittleEndian.PutUint32(errBody, ^uint32(0)) // -EPERM
	doneError := []byte{0xff, 0xff, 0xff, 0xff}
	for _, tc := range []struct {
		name   string
		data   []byte
		sender uint32
	}{
		{"short header", valid[:15], 0},
		{"short row", linuxMessage(20, 2, 42, 999, make([]byte, 71)), 0},
		{"wrong sequence", linuxMessage(20, 2, 43, 999, valid[16:]), 0},
		{"foreign sender", valid, 12},
		{"foreign header port", linuxMessage(20, 2, 42, 12, valid[16:]), 0},
		{"wrong family", linuxMessage(20, 2, 42, 999, linuxDiagBody(10, netip.MustParseAddr("::1"), 8080, 789)), 0},
		{"not multipart", linuxMessage(20, 0, 42, 999, valid[16:]), 0},
		{"error", linuxMessage(2, 0, 42, 999, errBody), 0},
		{"truncated error", linuxMessage(2, 0, 42, 999, make([]byte, 4)), 0},
		{"invalid acknowledged header", linuxMessage(2, 0, 42, 999, make([]byte, 20)), 0},
		{"done error", linuxMessage(3, 2, 42, 999, doneError), 0},
		{"truncated done", linuxMessage(3, 2, 42, 999, []byte{0}), 0},
		{"interrupted", linuxMessage(3, 2|16, 42, 999, make([]byte, 4)), 0},
		{"invalid attribute", linuxMessage(20, 2, 42, 999, append(valid[16:], 3, 0, 1, 0)), 0},
		{"unaligned record", append(linuxMessage(1, 0, 42, 999, []byte{1})[:17], valid...), 0},
		{"data after done", append(linuxMessage(3, 2, 42, 999, make([]byte, 4)), valid...), 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := decodeLinuxNetlink(tc.data, "ipv4", 42, 999, tc.sender); err == nil {
				t.Fatal("accepted invalid dump")
			}
		})
	}
}

func TestLinuxDumpDiscardsFailedAttempt(t *testing.T) {
	for _, tc := range []struct {
		name      string
		failures  int
		wantError bool
		calls     int
	}{
		{"retry once", 1, false, 2}, {"retry bound", 4, true, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			got := collectLinuxDump("ipv4", func() ([]inspect.Socket, error) {
				calls++
				if calls <= tc.failures {
					return []inspect.Socket{{ID: "discard"}}, &linuxDiagError{code: "collection_failed", retry: true}
				}
				return []inspect.Socket{{ID: "final"}}, nil
			})
			if (got.Error != nil) != tc.wantError || calls != tc.calls {
				t.Fatalf("result=%+v calls=%d", got, calls)
			}
			if tc.wantError && len(got.Sockets) != 0 {
				t.Fatal("retained partial sockets")
			}
			if !tc.wantError && !reflect.DeepEqual(got.Sockets, []inspect.Socket{{ID: "final"}}) {
				t.Fatalf("result=%+v", got)
			}
		})
	}
}

func TestLinuxRequestOnlyTCPListen(t *testing.T) {
	got := linuxDiagRequest("ipv6", 42)
	if len(got) != 72 || binary.LittleEndian.Uint32(got) != 72 || binary.LittleEndian.Uint16(got[4:]) != 20 || binary.LittleEndian.Uint16(got[6:]) != 0x301 || binary.LittleEndian.Uint32(got[8:]) != 42 || got[16] != 10 || got[17] != 6 || binary.LittleEndian.Uint32(got[20:]) != 1<<10 {
		t.Fatalf("request=%x", got)
	}
}

func TestLinuxNetlinkKernelFailureRetryPolicy(t *testing.T) {
	for _, tc := range []struct {
		name  string
		errno int32
		retry bool
	}{
		{"interrupted", 4, true}, {"receive overflow", 105, true}, {"permission denied", 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := make([]byte, 20)
			binary.LittleEndian.PutUint32(body, uint32(-tc.errno))
			copy(body[4:], linuxDiagRequest("ipv4", 42)[:16])
			_, _, err := decodeLinuxNetlink(linuxMessage(2, 0x100, 42, 999, body), "ipv4", 42, 999, 0)
			d, ok := err.(*linuxDiagError)
			if !ok || d.errno != int64(tc.errno) || d.retry != tc.retry {
				t.Fatalf("error=%+v", err)
			}
		})
	}
}

func TestLinuxNetlinkAcknowledgement(t *testing.T) {
	body := make([]byte, 20)
	copy(body[4:], linuxDiagRequest("ipv4", 42)[:16])
	rows, done, err := decodeLinuxNetlink(linuxMessage(2, 0, 42, 999, body), "ipv4", 42, 999, 0)
	if err != nil || done || len(rows) != 0 {
		t.Fatalf("rows=%v done=%v error=%v", rows, done, err)
	}
}
