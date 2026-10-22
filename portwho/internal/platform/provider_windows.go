//go:build windows && amd64

package platform

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/w00j00ng/goproj/portwho/internal/inspect"
	"golang.org/x/sys/windows"
)

type windowsProvider struct{}

func New() inspect.Provider { return windowsProvider{} }
func (windowsProvider) Runtime() inspect.Runtime {
	return inspect.Runtime{OS: runtime.GOOS, Arch: runtime.GOARCH, Backend: "windows_iphelper", SocketScope: "kernel_tcp", NetworkScope: "host", PIDScope: "os_reported"}
}

var extendedTCPTable = windows.NewLazySystemDLL("iphlpapi.dll").NewProc("GetExtendedTcpTable")

func (windowsProvider) Collect(families []string) map[string]inspect.FamilyResult {
	result := map[string]inspect.FamilyResult{}
	for _, family := range families {
		if err := extendedTCPTable.Find(); err != nil {
			result[family] = inspect.FamilyResult{Error: &inspect.Diagnostic{Stage: "platform", Family: &family, Code: "backend_unavailable", Message: "IP Helper TCP API is unavailable"}}
			continue
		}
		af := uintptr(windows.AF_INET)
		if family == "ipv6" {
			af = windows.AF_INET6
		}
		result[family] = collectWindowsTable(family, func(buf []byte, size *uint32) uint32 {
			var address uintptr
			if len(buf) > 0 {
				address = uintptr(unsafe.Pointer(&buf[0]))
			}
			// TCP_TABLE_OWNER_PID_LISTENER = 3, Reserved = 0.
			rc, _, _ := extendedTCPTable.Call(address, uintptr(unsafe.Pointer(size)), 0, af, 3, 0)
			runtime.KeepAlive(buf)
			return uint32(rc)
		})
	}
	return result
}

func (windowsProvider) Resolve(sockets []inspect.Socket) (map[string][]uint32, []string) {
	owners := map[string][]uint32{}
	for _, s := range sockets {
		owners[s.ID] = s.PIDs
	}
	return owners, nil
}

func (windowsProvider) ImagePath(pid uint32) (string, error) {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(handle)
	return queryWindowsImage(func(buf []uint16, n *uint32) error {
		return windows.QueryFullProcessImageName(handle, 0, &buf[0], n)
	})
}

func queryWindowsImage(query func([]uint16, *uint32) error) (string, error) {
	for size := uint32(260); ; {
		buf := make([]uint16, size)
		n := size
		err := query(buf, &n)
		if err == nil {
			if n == 0 || n > size {
				return "", fmt.Errorf("invalid process image length")
			}
			return windows.UTF16ToString(buf[:n]), nil
		}
		if err != windows.ERROR_INSUFFICIENT_BUFFER || size == 32768 {
			return "", err
		}
		size *= 2
		if size > 32768 {
			size = 32768
		}
	}
}
