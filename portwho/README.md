# portwho

Inspect local TCP LISTEN sockets and the process owners that the operating
system can associate with them. This is an independent Go module.

```text
portwho 8080
portwho 8080 5432 --json
portwho --all --family ipv6 --path
portwho 8080 --pids
portwho --help
portwho --version
```

The implementation follows [specification 0.2](docs/spec-v0.2.md). First-release
verification is in progress; see [the verification record](docs/verification.md)
for actual environments and outstanding release requirements.

## Install and build

Published binaries require no Go installation or C compiler. The intended
release targets are Windows 11 amd64, Linux amd64/arm64 with procfs and
NETLINK_SOCK_DIAG (Ubuntu 24.04 LTS baseline), and macOS 14+ Intel/Apple Silicon.
Release assets and checksums must be produced after native verification on
each target. This checkout does not establish that a published release exists.

Source builds require Go 1.26 or later. From this directory:

```sh
go build -o portwho .
go install .
```

On Windows, use `go build -o portwho.exe .`. Windows and Linux support
`CGO_ENABLED=0`. macOS requires `CGO_ENABLED=1`, a compatible C compiler and
Xcode Command Line Tools/macOS SDK. The macOS executable links system libraries;
it is not fully static. A Darwin build with cgo disabled supports help/version
but queries return `backend_unavailable` and exit 3; do not distribute that build
as a supported macOS binary.

After publication, source installation will have this form:

```sh
go install github.com/w00j00ng/goproj/portwho@latest
```

## Arguments and options

```text
portwho <port> [<port>...] [--family both|ipv4|ipv6] [--json|--pids] [--path]
portwho --all            [--family both|ipv4|ipv6] [--json|--pids] [--path]
```

Options can precede or follow ports. Ports are ASCII decimal numbers from
1 to 65535. Leading zeroes are accepted, duplicates removed, and ports sorted.
Multiple ports match with OR. Ranges, service names, hostnames and comma-separated
lists are rejected before any OS query. Input is never read from stdin.

| Option | Meaning |
| --- | --- |
| `--all` | Inspect all TCP LISTEN ports; cannot be combined with ports |
| `--family both\|ipv4\|ipv6` | Socket family; default `both`, lowercase only |
| `--json` | One JSON object with schema version 2 and LF |
| `--pids` | Sorted unique observed PIDs, one per line; skips image metadata |
| `--path` | Include full executable image paths |
| `-h`, `--help` | Help without OS queries |
| `--version` | `portwho <version>` without OS queries |

`--json` and `--pids` are exclusive. `--path` and `--pids` are exclusive.
Use JSON or PID mode for scripts; table spacing is not an automation contract.

## Results and exit codes

An endpoint groups observed raw sockets by protocol, socket family, IP address,
scope ID and port. Each endpoint has a sorted, unique `owners` array. Shared
descriptors may yield several PIDs, while independent SO_REUSEPORT sockets can
yield one endpoint. PID zero reported by an OS is retained as an actual value.

| `ownership_status` | Meaning |
| --- | --- |
| `resolved` | Every observed raw socket in the endpoint has at least one confirmed PID |
| `partial` | Only some observed raw sockets have a confirmed PID |
| `unresolved` | No observed raw socket has a confirmed PID; `owners` is `[]` |

`resolved` does not promise a complete list of every inherited handle/FD holder
or identify the original binder. Known visibility limits are recorded separately
in `ownership.limitations`; these alone do not cause exit 3. A missing owner is
never fabricated as PID 0, -1 or null.

| Exit | Meaning |
| --- | --- |
| 0 | Matching endpoints, complete requested socket collection, all endpoints resolved |
| 1 | Complete requested socket collection, no matching endpoints |
| 2 | Invalid arguments/options; stdout empty and no OS query |
| 3 | Incomplete socket collection/ownership, unsupported backend, or output failure |

Table and JSON preserve observed endpoints even when ownership or image metadata
cannot be read. An image-only failure keeps the confirmed PID and does not change
exit 0. `--pids` writes zero stdout bytes on collection or ownership failure;
check its exit code before consuming the list. An output failure can leave bytes
already written and always returns 3.

An empty table prints `No matching TCP listeners found.` only after complete
collection. An empty JSON result retains `listeners: []`; `--all` uses
`query.ports: null`. Empty output does not establish that a port can be bound.
IPv6 wildcard listeners do not generate IPv4 rows. IPv6 scope IDs remain separate
in JSON and appear as `[address%scope_id]:port` in the table when nonzero.

Metadata comes from the executable image path's basename, not a mutable process
label or command line. `process.path` is null unless `--path` is requested and
the query succeeds. Metadata failure yields null name/path, `unavailable`, and
`access_denied` or `os_error`; confirmed PIDs remain. Unix invalid filename bytes
become UTF-8 replacement characters. Table control characters are escaped.

Output is UTF-8 without BOM, with LF and no colors/pager. JSON stdout contains
only the object; core errors also have short stderr diagnostics. Metadata and
visibility warnings stay in JSON fields in JSON mode. See the specification for
all fields and error enums. Collection timestamps cover socket collection only;
PID/FD/socket reuse and observations at different times prevent an atomic view.

## Operating-system scope

| OS | Backend | Scope and owner interpretation |
| --- | --- | --- |
| Windows | IP Helper `GetExtendedTcpTable` | Host kernel TCP, API-reported PID (`reported_owner`) |
| Linux | `NETLINK_SOCK_DIAG` + procfs FD links | Current network namespace, visible procfs FD holders (`fd_holder`) |
| macOS | BSD `pcblist64` + libproc | Host BSD TCP table, visible FD holders (`fd_holder`) |

Linux retains inode/cookie identity while scanning visible PID/FD directories.
It does not use deprecated `/proc/net/tcp` as a fallback. The Netlink collector
uses at most three attempts with a three-second receive deadline per attempt and
bounded buffers. Restricted procfs or inaccessible FDs can leave ownership
unresolved without removing socket observations.

macOS joins socket/PCB opaque tokens, generation and endpoint identity rather
than guessing from address/port. SDK-defined native structures are decoded in a
small C shim. It does not inventory Skywalk/userland flows outside the BSD table.
The source-field mapping was inspected in Apple XNU commit
`f6217f891ac0bb64f3d375211650a4c1ff8ca1ea`; SDK/OS/CPU verification remains a
release requirement.

Windows/macOS query the host OS. A Docker Desktop published port can belong to a
host forwarding process; its PID is not converted to a container PID. A Linux
container or WSL Linux executable observes its own network namespace/procfs.
A Windows executable launched from WSL queries Windows. No automatic Docker,
WSL, namespace switch or host inventory occurs.

The tool uses current privileges and does not elevate, enable debug privileges,
terminate processes, bind/connect, resolve DNS, scan remote ports, run external
network tools, change configuration, or save/send results. It has no database,
configuration file, background helper, watch mode or cross-run cache.

## Verify

Run inside this module:

```sh
go test -mod=readonly -count=1 -timeout=2m ./...
go vet -mod=readonly ./...
go build -mod=readonly -o /tmp/portwho .
```

Choose a suitable temporary output path on Windows. Tests start child processes
on OS-assigned IPv4 and IPv6 loopback ports, run the actual binary, inspect
JSON/PID/`--all` output and verify the child listener survives. Native validation
requires local socket API access; restricted sandboxes can reject these calls.
Darwin native fixtures additionally require the macOS SDK and `xcrun clang`.
Cross-compilation and unsupported-build test skips do not count as native release
verification.
