//go:build darwin && cgo && (amd64 || arm64)

package platform

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/w00j00ng/goproj/portwho/internal/inspect"
)

func TestDarwinNativeBoundaryFixtures(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "native-fixtures")
	build := exec.Command("xcrun", "clang", "-std=c11", "-D_DARWIN_C_SOURCE", "-Wall", "-Wextra", "-Werror", "-I.",
		"-Dsysctlbyname=pw_test_sysctlbyname", "-Dproc_listpids=pw_test_proc_listpids", "-Dproc_pidinfo=pw_test_proc_pidinfo", "-Dproc_pidfdinfo=pw_test_proc_pidfdinfo", "-Dproc_pidpath=pw_test_proc_pidpath",
		"native_darwin.c", "testdata/native_darwin_test.c", "-lproc", "-o", binary)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build native fixtures: %v\n%s", err, output)
	}
	if output, err := exec.Command(binary).CombinedOutput(); err != nil {
		t.Fatalf("native fixtures: %v\n%s", err, output)
	}
}

func TestDarwinListenerRetainsKernelIdentityAndResolvesOwnPID(t *testing.T) {
	for _, network := range []string{"tcp4", "tcp6"} {
		t.Run(network, func(t *testing.T) {
			address, family := "127.0.0.1:0", "ipv4"
			if network == "tcp6" {
				address, family = "[::1]:0", "ipv6"
			}
			listener, err := net.Listen(network, address)
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			provider := New()
			result := provider.Collect([]string{family})[family]
			if result.Error != nil {
				t.Fatalf("collect: %+v", result.Error)
			}
			port := uint16(listener.Addr().(*net.TCPAddr).Port)
			var selected []inspect.Socket
			for _, socket := range result.Sockets {
				if socket.Port == port {
					selected = append(selected, socket)
				}
			}
			if len(selected) != 1 {
				t.Fatalf("listeners on %d = %d, want 1", port, len(selected))
			}
			if _, valid := darwinSocketIdentity(selected[0]); !valid {
				t.Fatalf("listener lacks native identity: %+v", selected[0])
			}
			owners, limitations := provider.Resolve(selected)
			found := false
			for _, pid := range owners[selected[0].ID] {
				if pid == uint32(os.Getpid()) {
					found = true
				}
			}
			if !found {
				t.Fatalf("own PID missing: owners=%v limitations=%v", owners, limitations)
			}
			mismatched := make([]inspect.Socket, 4)
			for i := range mismatched {
				mismatched[i] = selected[0]
				mismatched[i].ID = []string{"socket-token", "pcb-token", "generation", "missing-token"}[i]
			}
			mismatched[0].SocketToken++
			mismatched[1].PCBToken++
			mismatched[2].Generation++
			mismatched[3].SocketToken = 0
			rejected, _ := provider.Resolve(mismatched)
			for _, socket := range mismatched {
				if len(rejected[socket.ID]) != 0 {
					t.Fatalf("same endpoint with changed %s received owners: %v", socket.ID, rejected[socket.ID])
				}
			}
			path, err := provider.ImagePath(uint32(os.Getpid()))
			if err != nil || path == "" {
				t.Fatalf("image path = %q, error = %v", path, err)
			}
		})
	}
}
