package platform

import (
	"encoding/binary"
	"fmt"
	"net/netip"

	"github.com/w00j00ng/goproj/portwho/internal/inspect"
)

type linuxDiagError struct {
	code    string
	message string
	errno   int64
	retry   bool
}

func (e *linuxDiagError) Error() string { return e.message }

func linuxDiagRequest(family string, seq uint32) []byte {
	b := make([]byte, 72) // nlmsghdr + inet_diag_req_v2
	binary.LittleEndian.PutUint32(b, uint32(len(b)))
	binary.LittleEndian.PutUint16(b[4:], 20)    // SOCK_DIAG_BY_FAMILY
	binary.LittleEndian.PutUint16(b[6:], 0x301) // REQUEST | DUMP
	binary.LittleEndian.PutUint32(b[8:], seq)
	b[16], b[17] = 2, 6 // AF_INET, IPPROTO_TCP
	if family == "ipv6" {
		b[16] = 10
	}
	binary.LittleEndian.PutUint32(b[20:], 1<<10) // TCP_LISTEN
	return b
}

func collectLinuxDump(family string, attempt func() ([]inspect.Socket, error)) inspect.FamilyResult {
	var last error
	for i := 0; i < 3; i++ {
		sockets, err := attempt()
		if err == nil {
			return inspect.FamilyResult{Sockets: sockets}
		}
		last = err
		if d, ok := err.(*linuxDiagError); !ok || !d.retry {
			break
		}
	}
	d := &inspect.Diagnostic{Stage: "collection", Family: &family, Code: "collection_failed", Message: last.Error()}
	if e, ok := last.(*linuxDiagError); ok {
		d.Code = e.code
		if e.errno != 0 {
			n := e.errno
			d.OSCode = &n
		}
		if e.retry {
			d.Code = "resource_limit"
			d.Message = "SOCK_DIAG dump did not complete after 3 attempts: " + e.message
		}
	}
	return inspect.FamilyResult{Error: d}
}

func decodeLinuxNetlink(data []byte, family string, seq, portID, sender uint32) ([]inspect.Socket, bool, error) {
	invalid := func(message string) ([]inspect.Socket, bool, error) {
		return nil, false, &linuxDiagError{code: "invalid_os_data", message: message}
	}
	if sender != 0 {
		return invalid("SOCK_DIAG reply did not come from the kernel")
	}
	if len(data) == 0 || len(data) > maxNativeBuffer {
		return invalid("invalid SOCK_DIAG datagram size")
	}
	sockets := []inspect.Socket{}
	done := false
	for len(data) > 0 {
		if done {
			return invalid("SOCK_DIAG data follows NLMSG_DONE")
		}
		if len(data) < 16 {
			return invalid("truncated Netlink header")
		}
		n := uint64(binary.LittleEndian.Uint32(data))
		if n < 16 || n > uint64(len(data)) {
			return invalid("invalid Netlink message length")
		}
		aligned := (n + 3) &^ uint64(3)
		if aligned > uint64(len(data)) {
			return invalid("truncated Netlink alignment")
		}
		if binary.LittleEndian.Uint32(data[8:]) != seq {
			return invalid("SOCK_DIAG sequence mismatch")
		}
		headerPID := binary.LittleEndian.Uint32(data[12:])
		if headerPID != 0 && headerPID != portID {
			return invalid("SOCK_DIAG reply port mismatch")
		}
		flags := binary.LittleEndian.Uint16(data[6:])
		if flags&16 != 0 {
			return nil, false, &linuxDiagError{code: "collection_failed", message: "SOCK_DIAG dump interrupted", retry: true}
		}
		body := data[16:int(n)]
		switch binary.LittleEndian.Uint16(data[4:]) {
		case 1: // NLMSG_NOOP
		case 2: // NLMSG_ERROR
			if len(body) < 20 {
				return invalid("truncated NLMSG_ERROR")
			}
			originalLength := uint64(binary.LittleEndian.Uint32(body[4:]))
			if originalLength < 16 || originalLength > maxNativeBuffer || binary.LittleEndian.Uint16(body[8:]) != 20 || binary.LittleEndian.Uint32(body[12:]) != seq {
				return invalid("invalid NLMSG_ERROR request header")
			}
			attributeOffset := uint64(20)
			if int32(binary.LittleEndian.Uint32(body)) < 0 && flags&0x100 == 0 { // NLM_F_CAPPED omits the original payload.
				attributeOffset = 4 + (originalLength+3)&^uint64(3)
			}
			if attributeOffset > uint64(len(body)) {
				return invalid("truncated NLMSG_ERROR original request")
			}
			if flags&0x200 != 0 { // NLM_F_ACK_TLVS
				if !linuxAttributesValid(body[int(attributeOffset):]) {
					return invalid("invalid NLMSG_ERROR attributes")
				}
			} else if attributeOffset != uint64(len(body)) {
				return invalid("unexpected NLMSG_ERROR payload")
			}
			if err := linuxNetlinkStatus(body); err != nil {
				return nil, false, err
			}
		case 3: // NLMSG_DONE, with optional status and extended ACK attributes
			if len(body) != 0 {
				if len(body) < 4 {
					return invalid("truncated NLMSG_DONE status")
				}
				if err := linuxNetlinkStatus(body); err != nil {
					return nil, false, err
				}
				if !linuxAttributesValid(body[4:]) {
					return invalid("invalid NLMSG_DONE attributes")
				}
			}
			done = true
		case 4: // NLMSG_OVERRUN
			return nil, false, &linuxDiagError{code: "collection_failed", message: "SOCK_DIAG receive overrun", retry: true}
		case 20:
			if flags&2 == 0 {
				return invalid("SOCK_DIAG record is not multipart")
			}
			if len(body) < 72 {
				return invalid("truncated inet_diag_msg")
			}
			if !linuxAttributesValid(body[72:]) {
				return invalid("invalid inet_diag attributes")
			}
			af := byte(2)
			if family == "ipv6" {
				af = 10
			}
			if body[0] != af {
				return invalid("inet_diag address family mismatch")
			}
			if body[1] != 10 {
				break
			}
			s := inspect.Socket{Family: family, State: "LISTEN", Port: binary.BigEndian.Uint16(body[4:]), Inode: uint64(binary.LittleEndian.Uint32(body[68:])), UID: binary.LittleEndian.Uint32(body[64:]), Cookie: [2]uint32{binary.LittleEndian.Uint32(body[44:]), binary.LittleEndian.Uint32(body[48:])}}
			if af == 2 {
				s.Address = netip.AddrFrom4([4]byte(body[8:12]))
			} else {
				s.Address = netip.AddrFrom16([16]byte(body[8:24]))
				if s.Address.IsLinkLocalUnicast() || s.Address.IsLinkLocalMulticast() {
					s.ScopeID = binary.LittleEndian.Uint32(body[40:])
				}
			}
			s.ID = fmt.Sprintf("%s:%d:%08x:%08x", family, s.Inode, s.Cookie[0], s.Cookie[1])
			if len(sockets) >= maxNativeBuffer/256 {
				return nil, false, &linuxDiagError{code: "resource_limit", message: "SOCK_DIAG socket list exceeds memory limit"}
			}
			sockets = append(sockets, s)
		default:
			return invalid("unexpected SOCK_DIAG message type")
		}
		data = data[int(aligned):]
	}
	return sockets, done, nil
}

func linuxNetlinkStatus(body []byte) error {
	status := int32(binary.LittleEndian.Uint32(body))
	if status == 0 {
		return nil
	}
	if status > 0 {
		return &linuxDiagError{code: "invalid_os_data", message: "invalid positive Netlink error"}
	}
	n := -int64(status)
	return &linuxDiagError{code: "collection_failed", message: fmt.Sprintf("SOCK_DIAG kernel error %d", n), errno: n, retry: n == 4 || n == 105}
}

func linuxAttributesValid(data []byte) bool {
	for len(data) > 0 {
		if len(data) < 4 {
			return false
		}
		n := int(binary.LittleEndian.Uint16(data))
		if n < 4 || n > len(data) {
			return false
		}
		n = (n + 3) &^ 3
		if n > len(data) {
			return false
		}
		data = data[n:]
	}
	return true
}
