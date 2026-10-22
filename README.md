# goproj

Small, independent Go command-line tools for development. Each directory has
its own `go.mod`; install and validate each module separately.

| Module | Purpose | Minimum Go |
| --- | --- | --- |
| [bcrypter](bcrypter/README.md) | Generate and verify bcrypt password hashes | 1.20 |
| [scrypter](scrypter/README.md) | Generate and verify Better Auth default scrypt hashes within documented Unicode limits | 1.25.0 |
| [helloworld](helloworld/README.md) | Inspect HTTP requests with local defaults and bounded, opt-in body logging | 1.27.3 |
| [jumin](jumin/README.md) | Generate synthetic resident-number-shaped test values | 1.23.3 |
| [jwter](jwter/README.md) | Inspect compact JWTs; sign and verify HS256/ES256 tokens | 1.21 |
| [timetool](timetool/README.md) | Convert Unix seconds/milliseconds, RFC3339, and time zones | 1.27.3 |
| [portwho](portwho/README.md) | Inspect local TCP listeners and observed process owners; release verification in progress | 1.26.0 |

## Install

```sh
go install github.com/w00j00ng/goproj/bcrypter@latest
go install github.com/w00j00ng/goproj/scrypter@latest
go install github.com/w00j00ng/goproj/helloworld@latest
go install github.com/w00j00ng/goproj/jumin@latest
go install github.com/w00j00ng/goproj/jwter@latest
go install github.com/w00j00ng/goproj/timetool@latest
```

To try changes in a local checkout before publication, run `go install .`
inside the selected module. Remote `@latest` does not include uncommitted changes.
Use a currently patched Go toolchain; the module minimum alone is not a promise
that every later toolchain release is free of known issues.

`portwho` can be built or installed from its local module directory. Its first
release is still awaiting the native validation recorded in its README; a
remote `portwho@latest` installation is not assumed to exist yet.

## Platforms and usage limits

The command-line and HTTP paths can run on Windows, Linux and macOS.
Clipboard access is optional: Windows uses the native API without CGO; Linux
requires CGO, a C compiler, X11 development headers (for example `libx11-dev`
on Debian/Ubuntu), and an accessible X11 display; macOS requires CGO and the
platform compiler/frameworks. Linux/macOS builds with `CGO_ENABLED=0` still
support hashing and verification, but `--clipboard` returns an error.
Other operating systems have no clipboard implementation in these tools.

`portwho` has native Windows/Linux collectors and a macOS BSD/libproc collector
requiring CGO and the macOS SDK for source builds. Its results cover the local
OS/network namespace and visible process owners. See its README for support
verification status, runtime scope, partial results and exit codes.

`helloworld` defaults to `127.0.0.1:1323`. Its `--log-body` option records raw
request data and may expose secrets. Masking is limited to documented headers.
`jumin` generates test values; it cannot establish issuance, identity, or
non-collision with a real person. See each module's README before use.

With no key, `jwter decode` only inspects the token and returns
`validation.valid: null` with exit code 0. Supply a matching secret or public key
to verify the signature and `exp`/`nbf` claims. Verification does not check
issuer/audience policy, revocation, or application permissions. See the
[jwter README](jwter/README.md) for input options and examples.

## Verify from a checkout

Run these commands inside **each** of `bcrypter`, `scrypter`, `helloworld`,
`jumin`, `jwter`, `timetool`, and `portwho` (there is no root Go module):

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
