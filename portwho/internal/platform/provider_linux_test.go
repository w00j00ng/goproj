//go:build linux && (amd64 || arm64)

package platform

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/w00j00ng/goproj/portwho/internal/inspect"
	"golang.org/x/sys/unix"
)

func TestLinuxInheritedSocketChild(t *testing.T) {
	if os.Getenv("PORTWHO_SHARED_CHILD") != "1" {
		return
	}
	f := os.NewFile(3, "inherited-listener")
	defer f.Close()
	if _, err := f.Stat(); err != nil {
		t.Fatal(err)
	}
	fmt.Fprintln(os.Stdout, "ready")
	io.Copy(io.Discard, os.Stdin)
}

func TestLinuxReceiveDeadline(t *testing.T) {
	fd, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, unix.NETLINK_SOCK_DIAG)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(fd)
	if err := unix.Bind(fd, &unix.SockaddrNetlink{Family: unix.AF_NETLINK}); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, _, err = receiveLinuxNetlink(fd, start.Add(20*time.Millisecond))
	d, ok := err.(*linuxDiagError)
	if !ok || d.errno != int64(unix.ETIMEDOUT) || time.Since(start) > time.Second {
		t.Fatalf("elapsed=%v error=%v", time.Since(start), err)
	}
}

func TestLinuxInheritedSocketHasParentAndChildOwners(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	f, err := ln.(*net.TCPListener).File()
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestLinuxInheritedSocketChild$")
	child.Env = append(os.Environ(), "PORTWHO_SHARED_CHILD=1")
	child.ExtraFiles = []*os.File{f}
	stdin, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	child.Stderr = os.Stderr
	if err = child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		stdin.Close()
		if err := child.Wait(); err != nil {
			t.Error(err)
		}
	}()
	if line, err := bufio.NewReader(stdout).ReadString('\n'); err != nil || line != "ready\n" {
		t.Fatalf("child=%q error=%v", line, err)
	}
	p := linuxProvider{procRoot: "/proc"}
	result := p.Collect([]string{"ipv4"})["ipv4"]
	if result.Error != nil {
		t.Fatal(result.Error)
	}
	var selected []inspect.Socket
	for _, s := range result.Sockets {
		if int(s.Port) == ln.Addr().(*net.TCPAddr).Port {
			selected = append(selected, s)
		}
	}
	if len(selected) != 1 {
		t.Fatalf("sockets=%+v", selected)
	}
	owners, limitations := p.Resolve(selected)
	want := []uint32{uint32(os.Getpid()), uint32(child.Process.Pid)}
	slices.Sort(want)
	if !reflect.DeepEqual(owners[selected[0].ID], want) {
		t.Fatalf("owners=%v limitations=%v want=%v", owners, limitations, want)
	}
}

func TestLinuxNativeListenerAndDuplicateFD(t *testing.T) {
	for _, tc := range []struct{ network, address, family string }{
		{"tcp4", "127.0.0.1:0", "ipv4"}, {"tcp6", "[::1]:0", "ipv6"},
	} {
		t.Run(tc.family, func(t *testing.T) {
			ln, err := net.Listen(tc.network, tc.address)
			if err != nil {
				t.Fatal(err)
			}
			defer ln.Close()
			duplicate, err := ln.(*net.TCPListener).File()
			if err != nil {
				t.Fatal(err)
			}
			defer duplicate.Close()
			p := linuxProvider{procRoot: "/proc"}
			result := p.Collect([]string{tc.family})[tc.family]
			if result.Error != nil {
				t.Fatal(result.Error)
			}
			var selected []inspect.Socket
			for _, s := range result.Sockets {
				if int(s.Port) == ln.Addr().(*net.TCPAddr).Port {
					selected = append(selected, s)
				}
			}
			if len(selected) != 1 || selected[0].Inode == 0 {
				t.Fatalf("sockets=%+v", selected)
			}
			owners, limitations := p.Resolve(selected)
			want := []uint32{uint32(os.Getpid())}
			if !reflect.DeepEqual(owners[selected[0].ID], want) {
				t.Fatalf("owners=%v limitations=%v", owners, limitations)
			}
			image, err := p.ImagePath(uint32(os.Getpid()))
			if err != nil || image == "" {
				t.Fatalf("image=%q error=%v", image, err)
			}
		})
	}
}

func TestLinuxProcSharedOwnersAndUnknownSocket(t *testing.T) {
	root := t.TempDir()
	for _, pid := range []uint32{101, 102} {
		path := filepath.Join(root, strconv.FormatUint(uint64(pid), 10))
		if err := os.MkdirAll(filepath.Join(path, "fd"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "stat"), []byte(linuxStat(pid, 123)), 0600); err != nil {
			t.Fatal(err)
		}
		for _, fd := range []string{"3", "4"} {
			if err := os.Symlink("socket:[789]", filepath.Join(path, "fd", fd)); err != nil {
				t.Fatal(err)
			}
		}
	}
	p := linuxProvider{procRoot: root}
	input := []inspect.Socket{{ID: "shared", Inode: 789}, {ID: "unknown", Inode: 999}}
	got, _ := p.Resolve(input)
	if !reflect.DeepEqual(got["shared"], []uint32{101, 102}) || len(got["unknown"]) != 0 {
		t.Fatalf("owners=%v", got)
	}
}

func TestLinuxProcVisibilityRestrictions(t *testing.T) {
	for _, tc := range []struct {
		name            string
		hidepid, denied bool
	}{
		{"hidepid", true, false}, {"denied FD directory", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.denied && os.Geteuid() == 0 {
				t.Skip("root bypasses directory permission checks")
			}
			root := t.TempDir()
			pidDir := filepath.Join(root, "101")
			if err := os.MkdirAll(filepath.Join(pidDir, "fd"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(pidDir, "stat"), []byte(linuxStat(101, 123)), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("socket:[789]", filepath.Join(pidDir, "fd", "3")); err != nil {
				t.Fatal(err)
			}
			if tc.hidepid {
				if err := os.Mkdir(filepath.Join(root, "self"), 0700); err != nil {
					t.Fatal(err)
				}
				mountinfo := fmt.Sprintf("1 2 0:1 / %s rw - proc proc rw,hidepid=2\n", root)
				if err := os.WriteFile(filepath.Join(root, "self", "mountinfo"), []byte(mountinfo), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if tc.denied {
				if err := os.Chmod(filepath.Join(pidDir, "fd"), 0); err != nil {
					t.Fatal(err)
				}
				defer os.Chmod(filepath.Join(pidDir, "fd"), 0700)
			}
			p := linuxProvider{procRoot: root}
			owners, limitations := p.Resolve([]inspect.Socket{{ID: "socket", Inode: 789}})
			if tc.hidepid && (!slices.Contains(limitations, "restricted_procfs") || !reflect.DeepEqual(owners["socket"], []uint32{101})) {
				t.Fatalf("owners=%v limitations=%v", owners, limitations)
			}
			if tc.denied && (!slices.Contains(limitations, "permission_denied") || len(owners["socket"]) != 0) {
				t.Fatalf("owners=%v limitations=%v", owners, limitations)
			}
		})
	}
}

func TestLinuxNamespaceSocketChild(t *testing.T) {
	if os.Getenv("PORTWHO_NAMESPACE_CHILD") != "1" {
		return
	}
	ln, err := net.Listen("tcp4", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	f, err := ln.(*net.TCPListener).File()
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var stat unix.Stat_t
	if err := unix.Fstat(int(f.Fd()), &stat); err != nil {
		t.Fatal(err)
	}
	fmt.Fprintln(os.Stdout, stat.Ino)
	io.Copy(io.Discard, os.Stdin)
}

func TestLinuxDoesNotMixAnotherNetworkNamespace(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestLinuxNamespaceSocketChild$")
	child.Env = append(os.Environ(), "PORTWHO_NAMESPACE_CHILD=1")
	child.SysProcAttr = &syscall.SysProcAttr{Cloneflags: unix.CLONE_NEWNET}
	stdin, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	child.Stderr = os.Stderr
	if err := child.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) {
			t.Skip("creating another network namespace requires CAP_SYS_ADMIN")
		}
		t.Fatal(err)
	}
	defer func() {
		stdin.Close()
		if err := child.Wait(); err != nil {
			t.Error(err)
		}
	}()
	var inode uint64
	if _, err := fmt.Fscanln(bufio.NewReader(stdout), &inode); err != nil {
		t.Fatal(err)
	}
	result := New().Collect([]string{"ipv4"})["ipv4"]
	if result.Error != nil {
		t.Fatal(result.Error)
	}
	for _, s := range result.Sockets {
		if s.Inode == inode {
			t.Fatalf("socket from other network namespace included: %+v", s)
		}
	}
}

func TestLinuxProcessChangeDiscardsOwner(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "fd"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("socket:[789]", filepath.Join(root, "fd", "3")); err != nil {
		t.Fatal(err)
	}
	dir, err := os.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	calls := uint64(0)
	got, limitations := scanLinuxPID(dir, 101, map[uint64]bool{789: true}, func(*os.File, uint32) (uint64, error) { calls++; return calls, nil })
	if len(got) != 0 || !slices.Contains(limitations, "process_changed") {
		t.Fatalf("owners=%v limitations=%v", got, limitations)
	}
}

func TestLinuxReuseportKeepsSocketIdentity(t *testing.T) {
	lc := net.ListenConfig{Control: func(_, _ string, conn syscall.RawConn) error {
		var sockerr error
		if err := conn.Control(func(fd uintptr) { sockerr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1) }); err != nil {
			return err
		}
		return sockerr
	}}
	first, err := lc.Listen(context.Background(), "tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := lc.Listen(context.Background(), "tcp4", first.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	p := linuxProvider{procRoot: "/proc"}
	result := p.Collect([]string{"ipv4"})["ipv4"]
	if result.Error != nil {
		t.Fatal(result.Error)
	}
	var selected []inspect.Socket
	for _, s := range result.Sockets {
		if int(s.Port) == first.Addr().(*net.TCPAddr).Port {
			selected = append(selected, s)
		}
	}
	if len(selected) != 2 || selected[0].ID == selected[1].ID || selected[0].Inode == selected[1].Inode {
		t.Fatalf("sockets=%+v", selected)
	}
	owners, _ := p.Resolve(selected)
	for _, s := range selected {
		if !reflect.DeepEqual(owners[s.ID], []uint32{uint32(os.Getpid())}) {
			t.Fatal(fmt.Sprintf("owners=%v", owners))
		}
	}
}
