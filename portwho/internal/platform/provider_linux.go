//go:build linux && (amd64 || arm64)

package platform

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"time"

	"github.com/w00j00ng/goproj/portwho/internal/inspect"
	"golang.org/x/sys/unix"
)

type linuxProvider struct{ procRoot string }

func New() inspect.Provider { return linuxProvider{procRoot: "/proc"} }

func (linuxProvider) Runtime() inspect.Runtime {
	return inspect.Runtime{OS: runtime.GOOS, Arch: runtime.GOARCH, Backend: "linux_sock_diag", SocketScope: "kernel_tcp", NetworkScope: "current_network_namespace", PIDScope: "procfs_visible"}
}

func (linuxProvider) Collect(families []string) map[string]inspect.FamilyResult {
	result := make(map[string]inspect.FamilyResult, len(families))
	for _, family := range families {
		result[family] = collectLinuxDump(family, func() ([]inspect.Socket, error) { return linuxDumpAttempt(family) })
	}
	return result
}

func linuxDumpAttempt(family string) ([]inspect.Socket, error) {
	fd, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, unix.NETLINK_SOCK_DIAG)
	if err != nil {
		return nil, linuxReceiveError(err)
	}
	defer unix.Close(fd)
	if err = unix.Bind(fd, &unix.SockaddrNetlink{Family: unix.AF_NETLINK}); err != nil {
		return nil, linuxReceiveError(err)
	}
	local, err := unix.Getsockname(fd)
	if err != nil {
		return nil, linuxReceiveError(err)
	}
	portID := local.(*unix.SockaddrNetlink).Pid
	if err = unix.Sendto(fd, linuxDiagRequest(family, 1), 0, &unix.SockaddrNetlink{Family: unix.AF_NETLINK}); err != nil {
		return nil, linuxReceiveError(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	sockets := []inspect.Socket{}
	for {
		data, sender, err := receiveLinuxNetlink(fd, deadline)
		if err != nil {
			return nil, err
		}
		rows, done, err := decodeLinuxNetlink(data, family, 1, portID, sender)
		if err != nil {
			return nil, err
		}
		if len(rows) > maxNativeBuffer/256-len(sockets) {
			return nil, &linuxDiagError{code: "resource_limit", message: "SOCK_DIAG socket list exceeds memory limit"}
		}
		sockets = append(sockets, rows...)
		if done {
			return sockets, nil
		}
	}
}

func receiveLinuxNetlink(fd int, deadline time.Time) ([]byte, uint32, error) {
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, 0, &linuxDiagError{code: "collection_failed", message: "SOCK_DIAG receive deadline exceeded", errno: int64(unix.ETIMEDOUT)}
		}
		poll := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		n, err := unix.Poll(poll, int((remaining+time.Millisecond-1)/time.Millisecond))
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return nil, 0, linuxReceiveError(err)
		}
		if n == 0 {
			continue
		}
		if poll[0].Revents&unix.POLLNVAL != 0 {
			return nil, 0, linuxReceiveError(unix.EBADF)
		}
		var peek [1]byte
		n, _, _, _, err = unix.Recvmsg(fd, peek[:], nil, unix.MSG_PEEK|unix.MSG_TRUNC|unix.MSG_DONTWAIT)
		if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return nil, 0, linuxReceiveError(err)
		}
		if n <= 0 || n > maxNativeBuffer {
			return nil, 0, &linuxDiagError{code: "resource_limit", message: "SOCK_DIAG datagram exceeds buffer limit"}
		}
		data := make([]byte, n)
		n, _, flags, from, err := unix.Recvmsg(fd, data, nil, unix.MSG_DONTWAIT)
		if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return nil, 0, linuxReceiveError(err)
		}
		if flags&unix.MSG_TRUNC != 0 || n > len(data) {
			return nil, 0, &linuxDiagError{code: "collection_failed", message: "truncated SOCK_DIAG datagram", retry: true}
		}
		sender, ok := from.(*unix.SockaddrNetlink)
		if !ok {
			return nil, 0, &linuxDiagError{code: "invalid_os_data", message: "invalid SOCK_DIAG sender address"}
		}
		return data[:n], sender.Pid, nil
	}
}

func linuxReceiveError(err error) error {
	n := int64(0)
	var errno unix.Errno
	if errors.As(err, &errno) {
		n = int64(errno)
	}
	return &linuxDiagError{code: "collection_failed", message: "SOCK_DIAG: " + err.Error(), errno: n, retry: errors.Is(err, unix.ENOBUFS)}
}

func (p linuxProvider) Resolve(sockets []inspect.Socket) (map[string][]uint32, []string) {
	owners := make(map[string][]uint32)
	if len(sockets) == 0 {
		return owners, []string{}
	}
	wanted := make(map[uint64]bool)
	for _, s := range sockets {
		if s.Inode != 0 {
			wanted[s.Inode] = true
		}
	}
	limitations := make(map[string]bool)
	root, err := os.Open(p.procRoot)
	if err != nil {
		return owners, []string{linuxProcError(err)}
	}
	defer root.Close()
	if restricted, err := linuxProcRestricted(p.procRoot); err == nil && restricted {
		limitations["restricted_procfs"] = true
	} else if err != nil && !errors.Is(err, unix.ENOENT) {
		limitations[linuxProcError(err)] = true
	}
	selfNS, selfErr := readLinuxLinkAt(int(root.Fd()), "self/ns/pid")
	initNS, initErr := readLinuxLinkAt(int(root.Fd()), "1/ns/pid")
	if selfErr == nil && initErr == nil && selfNS != initNS {
		limitations["namespace_visibility"] = true
	}
	inodePIDs := make(map[uint64][]uint32)
	count, mappings := 0, 0
	for {
		names, readErr := root.Readdirnames(256)
		for _, name := range names {
			n, err := strconv.ParseUint(name, 10, 32)
			if err != nil || n == 0 {
				continue
			}
			count++
			if count > maxNativeBuffer/32 {
				limitations["os_error"] = true
				break
			}
			pidFD, err := unix.Openat(int(root.Fd()), name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
			if err != nil {
				limitations[linuxProcError(err)] = true
				continue
			}
			dir := os.NewFile(uintptr(pidFD), name)
			inodes, limited := scanLinuxPID(dir, uint32(n), wanted, readLinuxStarttime)
			dir.Close()
			for _, limitation := range limited {
				limitations[limitation] = true
			}
			for inode := range inodes {
				mappings++
				if mappings > maxNativeBuffer/32 {
					limitations["os_error"] = true
					break
				}
				inodePIDs[inode] = append(inodePIDs[inode], uint32(n))
			}
			if mappings > maxNativeBuffer/32 {
				break
			}
		}
		if count > maxNativeBuffer/32 || mappings > maxNativeBuffer/32 {
			break
		}
		if readErr != nil {
			if readErr != io.EOF {
				limitations[linuxProcError(readErr)] = true
			}
			break
		}
	}
	for _, s := range sockets {
		if pids := inodePIDs[s.Inode]; len(pids) > 0 {
			slices.Sort(pids)
			owners[s.ID] = pids
		}
	}
	limited := make([]string, 0, len(limitations))
	for limitation := range limitations {
		limited = append(limited, limitation)
	}
	slices.Sort(limited)
	return owners, limited
}

func scanLinuxPID(dir *os.File, pid uint32, wanted map[uint64]bool, readStart func(*os.File, uint32) (uint64, error)) (map[uint64]bool, []string) {
	inodes := make(map[uint64]bool)
	before, err := readStart(dir, pid)
	if err != nil {
		return inodes, []string{linuxProcError(err)}
	}
	fd, err := unix.Openat(int(dir.Fd()), "fd", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return inodes, []string{linuxProcError(err)}
	}
	fds := os.NewFile(uintptr(fd), "fd")
	defer fds.Close()
	limitations := []string{}
	addLimitation := func(code string) {
		if !slices.Contains(limitations, code) {
			limitations = append(limitations, code)
		}
	}
	count := 0
	for {
		names, readErr := fds.Readdirnames(256)
		for _, name := range names {
			count++
			if count > maxNativeBuffer/32 {
				addLimitation("os_error")
				break
			}
			var buf [64]byte
			n, err := unix.Readlinkat(fd, name, buf[:])
			if err != nil {
				addLimitation(linuxProcError(err))
				continue
			}
			if n == len(buf) {
				continue
			} // Longer links cannot be socket:[uint64].
			if inode := linuxSocketInode(string(buf[:n])); wanted[inode] {
				inodes[inode] = true
			}
		}
		if count > maxNativeBuffer/32 {
			break
		}
		if readErr != nil {
			if readErr != io.EOF {
				addLimitation(linuxProcError(readErr))
			}
			break
		}
	}
	after, err := readStart(dir, pid)
	if err != nil {
		addLimitation(linuxProcError(err))
		return nil, limitations
	}
	if before != after {
		addLimitation("process_changed")
		return nil, limitations
	}
	return inodes, limitations
}

func readLinuxStarttime(dir *os.File, pid uint32) (uint64, error) {
	fd, err := unix.Openat(int(dir.Fd()), "stat", unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return 0, err
	}
	f := os.NewFile(uintptr(fd), "stat")
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (64<<10)+1))
	if err != nil {
		return 0, err
	}
	if len(data) > 64<<10 {
		return 0, fmt.Errorf("proc stat exceeds size limit")
	}
	return parseLinuxStarttime(data, pid)
}

func linuxProcRestricted(root string) (bool, error) {
	f, err := os.Open(filepath.Join(root, "self", "mountinfo"))
	if err != nil {
		return false, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(io.LimitReader(f, maxNativeBuffer+1))
	scanner.Buffer(make([]byte, 4096), 64<<10)
	consumed := 0
	for scanner.Scan() {
		consumed += len(scanner.Bytes()) + 1
		if consumed > maxNativeBuffer {
			return false, fmt.Errorf("proc mountinfo exceeds size limit")
		}
		if linuxRestrictedProcfs(scanner.Text(), root) {
			return true, nil
		}
	}
	return false, scanner.Err()
}

func linuxProcError(err error) string {
	switch {
	case errors.Is(err, unix.EACCES), errors.Is(err, unix.EPERM):
		return "permission_denied"
	case errors.Is(err, unix.ENOENT), errors.Is(err, unix.ESRCH), errors.Is(err, unix.EBADF):
		return "process_changed"
	default:
		return "os_error"
	}
}

func (p linuxProvider) ImagePath(pid uint32) (string, error) {
	fd, err := unix.Open(filepath.Join(p.procRoot, strconv.FormatUint(uint64(pid), 10)), unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return "", err
	}
	defer unix.Close(fd)
	return readLinuxLinkAt(fd, "exe")
}

func readLinuxLinkAt(fd int, path string) (string, error) {
	for size := 4096; size <= maxNativeBuffer; size *= 2 {
		buf := make([]byte, size)
		n, err := unix.Readlinkat(fd, path, buf)
		if err != nil {
			return "", err
		}
		if n < len(buf) {
			return string(buf[:n]), nil
		}
	}
	return "", fmt.Errorf("proc symlink exceeds size limit")
}
