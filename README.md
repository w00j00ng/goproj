# goproj

Small, independent Go command-line tools for development. Each directory has
its own `go.mod`; install and validate each module separately.

| Module | Purpose | Minimum Go |
| --- | --- | --- |
| [bcrypter](bcrypter/README.md) | Generate and verify bcrypt password hashes | 1.20 |
| [scrypter](scrypter/README.md) | Generate and verify Better Auth default scrypt hashes within documented Unicode limits | 1.25.0 |
| [helloworld](helloworld/README.md) | Inspect HTTP requests with local defaults and bounded, opt-in body logging | 1.27.3 |
| [jumin](jumin/README.md) | Generate synthetic resident-number-shaped test values | 1.23.3 |

## Install

```sh
go install github.com/w00j00ng/goproj/bcrypter@latest
go install github.com/w00j00ng/goproj/scrypter@latest
go install github.com/w00j00ng/goproj/helloworld@latest
go install github.com/w00j00ng/goproj/jumin@latest
```

To try changes in a local checkout before publication, run `go install .`
inside the selected module. Remote `@latest` does not include uncommitted changes.
Use a currently patched Go toolchain; the module minimum alone is not a promise
that every later toolchain release is free of known issues.

## Platforms and usage limits

The command-line and HTTP paths can run on Windows, Linux and macOS.
Clipboard access is optional: Windows uses the native API without CGO; Linux
requires CGO, a C compiler, X11 development headers (for example `libx11-dev`
on Debian/Ubuntu), and an accessible X11 display; macOS requires CGO and the
platform compiler/frameworks. Linux/macOS builds with `CGO_ENABLED=0` still
support hashing and verification, but `--clipboard` returns an error.
Other operating systems have no clipboard implementation in these tools.

`helloworld` defaults to `127.0.0.1:1323`. Its `--log-body` option records raw
request data and may expose secrets. Masking is limited to documented headers.
`jumin` generates test values; it cannot establish issuance, identity, or
non-collision with a real person. See each module's README before use.

## Verify from a checkout

Run these commands inside **each** of `bcrypter`, `scrypter`, `helloworld`, and
`jumin` (there is no root Go module):

```sh
go test -mod=readonly -count=1 -timeout=2m ./...
go vet -mod=readonly ./...
go build -mod=readonly .
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
```

Linux builds that do not need clipboard access can set `CGO_ENABLED=0`.
Unicode fixtures can be regenerated with
`node password/testdata/generate.mjs` from `scrypter`; their JSON records the
Node.js, ICU and Unicode versions. Normal tests do not need Node.js.

CI workflows are deferred; these commands are local verification instructions.
