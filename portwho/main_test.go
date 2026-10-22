package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/w00j00ng/goproj/portwho/internal/inspect"
	"github.com/w00j00ng/goproj/portwho/internal/platform"
)

type childListener struct {
	Port    int
	PID     uint32
	Address string
}

func TestListenerChild(t *testing.T) {
	family := os.Getenv("PORTWHO_TEST_LISTENER")
	if family == "" {
		return
	}
	address := "127.0.0.1:0"
	if family == "tcp6" {
		address = "[::1]:0"
	}
	l, err := net.Listen(family, address)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	defer l.Close()
	a := l.Addr().(*net.TCPAddr)
	if err := json.NewEncoder(os.Stdout).Encode(childListener{Port: a.Port, PID: uint32(os.Getpid()), Address: a.IP.String()}); err != nil {
		os.Exit(2)
	}
	// The child keeps the socket alive until its parent closes the pipe.
	_, _ = bufio.NewReader(os.Stdin).ReadByte()
}

func TestNativeCLI(t *testing.T) {
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("unsupported host")
	}
	if platform.New().Runtime().Backend == "unsupported" {
		t.Skip("native backend unavailable in this build; not release validation")
	}
	bin := filepath.Join(t.TempDir(), "portwho")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	build := exec.Command("go", "build", "-mod=readonly", "-o", bin, ".")
	if data, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, data)
	}
	for _, family := range []string{"tcp4", "tcp6"} {
		t.Run(family, func(t *testing.T) {
			self, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			child := exec.Command(self, "-test.run=^TestListenerChild$")
			child.Env = append(os.Environ(), "PORTWHO_TEST_LISTENER="+family)
			stdin, err := child.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			stdout, err := child.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			var childErr bytes.Buffer
			child.Stderr = &childErr
			if err := child.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() {
				stdin.Close()
				if err := child.Wait(); err != nil {
					t.Errorf("listener exited: %v %s", err, childErr.String())
				}
			}()
			var endpoint childListener
			ready := make(chan error, 1)
			go func() { ready <- json.NewDecoder(stdout).Decode(&endpoint) }()
			select {
			case err := <-ready:
				if err != nil {
					t.Fatalf("listener setup: %v %s", err, childErr.String())
				}
			case <-time.After(10 * time.Second):
				child.Process.Kill()
				t.Fatal("listener setup timed out")
			}
			queryFamily := "ipv4"
			if family == "tcp6" {
				queryFamily = "ipv6"
			}
			for _, mode := range []string{"json", "pids", "all"} {
				t.Run(mode, func(t *testing.T) {
					args := []string{strconv.Itoa(endpoint.Port), "--family", queryFamily, "--json", "--path"}
					if mode == "pids" {
						args = []string{strconv.Itoa(endpoint.Port), "--family", queryFamily, "--pids"}
					}
					if mode == "all" {
						args = []string{"--all", "--family", queryFamily, "--json"}
					}
					command := exec.Command(bin, args...)
					var out, errout bytes.Buffer
					command.Stdout = &out
					command.Stderr = &errout
					err := command.Run()
					if err != nil && (mode != "all" || command.ProcessState.ExitCode() != 3) {
						t.Fatalf("query %v: %v stderr=%s stdout=%s", args, err, errout.String(), out.String())
					}
					if mode == "pids" {
						if out.String() != fmt.Sprintf("%d\n", endpoint.PID) {
							t.Fatalf("pids=%q want %d", out.String(), endpoint.PID)
						}
						return
					}
					var result inspect.Result
					if err := json.Unmarshal(out.Bytes(), &result); err != nil {
						t.Fatal(err)
					}
					found := false
					for _, l := range result.Listeners {
						if l.Local.Port == uint16(endpoint.Port) && l.Local.Address == endpoint.Address && l.Family == queryFamily {
							for _, o := range l.Owners {
								if o.PID == endpoint.PID {
									found = true
									if o.Process.Status != "ok" || o.Process.Name == nil || !strings.Contains(*o.Process.Name, "portwho") {
										t.Fatalf("image=%+v", o.Process)
									}
									if mode == "json" && o.Process.Path == nil {
										t.Fatal("missing image path")
									}
								}
							}
						}
					}
					if !found {
						t.Fatalf("child endpoint not found: %+v stderr=%s", result, errout.String())
					}
				})
			}
			// Parent still controls a live child after inspection; the tool does not close its listener.
			connection, err := net.DialTimeout(family, net.JoinHostPort(endpoint.Address, strconv.Itoa(endpoint.Port)), time.Second)
			if err != nil {
				t.Fatalf("listener no longer alive: %v", err)
			}
			connection.Close()
		})
	}
}
