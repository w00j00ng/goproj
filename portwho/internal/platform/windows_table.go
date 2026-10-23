package platform

import (
	"encoding/binary"
	"fmt"
	"net/netip"

	"github.com/w00j00ng/goproj/portwho/internal/inspect"
)

const maxNativeBuffer = 64 << 20

func collectWindowsTable(family string, call func([]byte, *uint32) uint32) inspect.FamilyResult {
	failure := func(code, message string, rc uint32) inspect.FamilyResult {
		d := &inspect.Diagnostic{Stage: "collection", Family: &family, Code: code, Message: message}
		if rc != 0 {
			n := int64(rc)
			d.OSCode = &n
		}
		return inspect.FamilyResult{Error: d}
	}
	size := uint32(0)
	rc := call(nil, &size)
	if rc != 0 && rc != 122 {
		return failure("collection_failed", "GetExtendedTcpTable failed", rc)
	}
	for attempt := 0; attempt < 5; attempt++ {
		if size > maxNativeBuffer {
			return failure("resource_limit", "TCP table exceeds 64 MiB", 0)
		}
		if size < 4 {
			return failure("invalid_os_data", "invalid TCP table size", 0)
		}
		buf := make([]byte, int(size))
		n := size
		rc = call(buf, &n)
		if rc == 122 {
			size = n
			continue
		}
		if rc != 0 {
			return failure("collection_failed", "GetExtendedTcpTable failed", rc)
		}
		if n > uint32(len(buf)) || n < 4 {
			return failure("invalid_os_data", "invalid TCP table length", 0)
		}
		sockets, err := decodeWindowsTable(buf[:n], family)
		if err != nil {
			return failure("invalid_os_data", err.Error(), 0)
		}
		return inspect.FamilyResult{Sockets: sockets}
	}
	return failure("resource_limit", "TCP table acquisition exceeded 5 attempts", rc)
}

func decodeWindowsTable(data []byte, family string) ([]inspect.Socket, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("truncated TCP table header")
	}
	stride := 24
	if family == "ipv6" {
		stride = 56
	}
	count := binary.LittleEndian.Uint32(data)
	if uint64(count) > uint64((len(data)-4)/stride) {
		return nil, fmt.Errorf("TCP table count exceeds buffer length")
	}
	sockets := []inspect.Socket{}
	for i := 0; i < int(count); i++ {
		row := data[4+i*stride : 4+(i+1)*stride]
		s := inspect.Socket{ID: fmt.Sprintf("%s:%d", family, i), Family: family, State: "LISTEN"}
		if family == "ipv4" {
			if binary.LittleEndian.Uint32(row) != 2 {
				continue
			}
			s.Address = netip.AddrFrom4([4]byte(row[4:8]))
			s.Port = binary.BigEndian.Uint16(row[8:10])
			s.PIDs = []uint32{binary.LittleEndian.Uint32(row[20:24])}
		} else {
			if binary.LittleEndian.Uint32(row[48:52]) != 2 {
				continue
			}
			s.Address = netip.AddrFrom16([16]byte(row[:16]))
			s.ScopeID = binary.BigEndian.Uint32(row[16:20])
			s.Port = binary.BigEndian.Uint16(row[20:22])
			s.PIDs = []uint32{binary.LittleEndian.Uint32(row[52:56])}
		}
		sockets = append(sockets, s)
	}
	return sockets, nil
}
