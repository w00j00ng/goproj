package platform

import (
	"encoding/binary"
	"reflect"
	"testing"
)

func TestWindowsNativeTables(t *testing.T) {
	for _, tt := range []struct {
		family  string
		stride  int
		address string
		scope   uint32
	}{
		{"ipv4", 24, "127.0.0.1", 0}, {"ipv6", 56, "fe80::1", 12},
	} {
		t.Run(tt.family, func(t *testing.T) {
			data := make([]byte, 4+tt.stride)
			binary.LittleEndian.PutUint32(data, 1)
			row := data[4:]
			if tt.family == "ipv4" {
				binary.LittleEndian.PutUint32(row, 2)
				copy(row[4:8], []byte{127, 0, 0, 1})
				binary.BigEndian.PutUint16(row[8:10], 8080)
				binary.LittleEndian.PutUint32(row[20:], 42)
			} else {
				row[0] = 0xfe
				row[1] = 0x80
				row[15] = 1
				binary.BigEndian.PutUint32(row[16:], 12)
				binary.BigEndian.PutUint16(row[20:], 8080)
				binary.LittleEndian.PutUint32(row[48:], 2)
				binary.LittleEndian.PutUint32(row[52:], 42)
			}
			got, err := decodeWindowsTable(data, tt.family)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 || got[0].Address.String() != tt.address || got[0].ScopeID != tt.scope || got[0].Port != 8080 || !reflect.DeepEqual(got[0].PIDs, []uint32{42}) {
				t.Fatalf("got=%+v", got)
			}
			for _, bad := range [][]byte{nil, data[:3], data[:len(data)-1], {255, 255, 255, 255}} {
				if _, err := decodeWindowsTable(bad, tt.family); err == nil {
					t.Fatal("accepted truncated/overflowing table")
				}
			}
		})
	}
}

func TestWindowsBufferRaces(t *testing.T) {
	for _, tt := range []struct {
		name  string
		sizes []uint32
		codes []uint32
		want  string
		calls int
	}{
		{"normal empty", []uint32{4, 4}, []uint32{122, 0}, "", 2},
		{"table grew", []uint32{4, 28, 4}, []uint32{122, 122, 0}, "", 3},
		{"too large", []uint32{64<<20 + 1}, []uint32{122}, "resource_limit", 1},
		{"exhausted", []uint32{4, 4, 4, 4, 4, 4}, []uint32{122, 122, 122, 122, 122, 122}, "resource_limit", 6},
		{"bad success size", []uint32{4, 20}, []uint32{122, 0}, "invalid_os_data", 2},
		{"API error", []uint32{0}, []uint32{5}, "collection_failed", 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			got := collectWindowsTable("ipv4", func(_ []byte, size *uint32) uint32 { i := calls; calls++; *size = tt.sizes[i]; return tt.codes[i] })
			if calls != tt.calls {
				t.Fatalf("calls=%d want %d", calls, tt.calls)
			}
			if tt.want == "" {
				if got.Error != nil {
					t.Fatal(got.Error)
				}
			} else if got.Error == nil || got.Error.Code != tt.want {
				t.Fatalf("got=%+v", got)
			}
		})
	}
}
