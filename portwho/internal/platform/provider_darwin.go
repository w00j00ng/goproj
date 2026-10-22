//go:build darwin && cgo && (amd64 || arm64)

package platform

/*
#cgo CFLAGS: -std=c11 -D_DARWIN_C_SOURCE -Wall -Wextra
#cgo LDFLAGS: -lproc
#include "native_darwin.h"
*/
import "C"

import (
	"errors"
	"net/netip"
	"runtime"
	"slices"
	"strconv"
	"syscall"
	"unsafe"

	"github.com/w00j00ng/goproj/portwho/internal/inspect"
)

type darwinProvider struct{}

func New() inspect.Provider { return darwinProvider{} }

func (darwinProvider) Runtime() inspect.Runtime {
	return inspect.Runtime{OS: "darwin", Arch: runtime.GOARCH, Backend: "darwin_pcblist64", SocketScope: "bsd_tcp", NetworkScope: "host", PIDScope: "libproc_visible"}
}

func (darwinProvider) Collect(families []string) map[string]inspect.FamilyResult {
	results := make(map[string]inspect.FamilyResult, len(families))
	for _, family := range families {
		results[family] = inspect.FamilyResult{}
	}
	if len(families) == 0 {
		return results
	}
	var rows *C.pw_socket
	var count C.size_t
	var osCode C.int
	status := C.pw_collect(&rows, &count, &osCode)
	defer C.pw_free(unsafe.Pointer(rows))
	if status != C.PW_OK {
		code := "collection_failed"
		if status == C.PW_RESOURCE_LIMIT {
			code = "resource_limit"
		}
		if status == C.PW_INVALID_DATA {
			code = "invalid_os_data"
		}
		for _, family := range families {
			diagnostic := &inspect.Diagnostic{Stage: "collection", Family: &family, Code: code, Message: "Could not read a complete BSD TCP socket table."}
			if osCode != 0 {
				value := int64(osCode)
				diagnostic.OSCode = &value
			}
			results[family] = inspect.FamilyResult{Error: diagnostic}
		}
		return results
	}
	for index, native := range unsafe.Slice(rows, int(count)) {
		socket := socketFromDarwin(native)
		result, requested := results[socket.Family]
		if !requested {
			continue
		}
		socket.ID = "darwin:" + strconv.Itoa(index)
		result.Sockets = append(result.Sockets, socket)
		results[socket.Family] = result
	}
	return results
}

func socketFromDarwin(native C.pw_socket) inspect.Socket {
	var address [16]byte
	for i := range address {
		address[i] = byte(native.address[i])
	}
	family, ip := "ipv6", netip.AddrFrom16(address)
	if native.family == 4 {
		family, ip = "ipv4", netip.AddrFrom4([4]byte(address[:4]))
	}
	return inspect.Socket{Family: family, Address: ip, Port: uint16(native.port), ScopeID: uint32(native.scope_id), State: "LISTEN", SocketToken: uint64(native.socket_token), PCBToken: uint64(native.pcb_token), Generation: uint64(native.generation)}
}

func (darwinProvider) Resolve(sockets []inspect.Socket) (map[string][]uint32, []string) {
	owners := make(map[string][]uint32)
	targets := make(map[darwinIdentity][]string)
	for _, socket := range sockets {
		if key, valid := darwinSocketIdentity(socket); valid {
			targets[key] = append(targets[key], socket.ID)
		}
	}
	if len(targets) == 0 {
		return owners, nil
	}
	limitations := make(map[string]bool)
	var pids *C.int32_t
	var count C.size_t
	var osCode C.int
	status := C.pw_listpids(&pids, &count, &osCode)
	defer C.pw_free(unsafe.Pointer(pids))
	if status != C.PW_OK {
		limitations[darwinLimitation(osCode)] = true
	} else {
		for _, pid := range unsafe.Slice(pids, int(count)) {
			if pid <= 0 {
				continue
			}
			var fds *C.int32_t
			var fdCount C.size_t
			status = C.pw_listfds(pid, &fds, &fdCount, &osCode)
			if status != C.PW_OK {
				limitations[darwinLimitation(osCode)] = true
				continue
			}
			for _, fd := range unsafe.Slice(fds, int(fdCount)) {
				var native C.pw_socket
				status = C.pw_socketinfo(pid, fd, &native, &osCode)
				if status == C.PW_NOT_LISTENER {
					continue
				}
				if status != C.PW_OK {
					limitations[darwinLimitation(osCode)] = true
					continue
				}
				key, valid := darwinSocketIdentity(socketFromDarwin(native))
				if !valid {
					continue
				}
				for _, id := range targets[key] {
					if !slices.Contains(owners[id], uint32(pid)) {
						owners[id] = append(owners[id], uint32(pid))
					}
				}
			}
			C.pw_free(unsafe.Pointer(fds))
		}
	}
	for _, pids := range owners {
		slices.Sort(pids)
	}
	var codes []string
	for code := range limitations {
		codes = append(codes, code)
	}
	slices.Sort(codes)
	return owners, codes
}

func darwinLimitation(osCode C.int) string {
	switch syscall.Errno(osCode) {
	case syscall.EACCES, syscall.EPERM:
		return "permission_denied"
	case syscall.ESRCH, syscall.ENOENT, syscall.EBADF, syscall.ENOTSOCK:
		return "process_changed"
	default:
		return "os_error"
	}
}

func (darwinProvider) ImagePath(pid uint32) (string, error) {
	var path *C.char
	var osCode C.int
	status := C.pw_imagepath(C.uint32_t(pid), &path, &osCode)
	defer C.pw_free(unsafe.Pointer(path))
	if status != C.PW_OK {
		if osCode != 0 {
			return "", syscall.Errno(osCode)
		}
		return "", errors.New("could not read the process image path")
	}
	return C.GoString(path), nil
}
