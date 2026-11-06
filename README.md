# goproj

Small, independent Go command-line tools for development. Each directory has
its own `go.mod`; install and validate each module separately.

| Module | Purpose | Minimum Go |
| --- | --- | --- |
| [bcrypter](bcrypter/README.md) | Generate and verify bcrypt password hashes | 1.27.3 |
| [scrypter](scrypter/README.md) | Generate and verify Better Auth default scrypt hashes within documented Unicode limits | 1.27.3 |
| [helloworld](helloworld/README.md) | Inspect HTTP requests with local defaults and bounded, opt-in body logging | 1.27.3 |
| [jumin](jumin/README.md) | Generate synthetic resident-number-shaped test values | 1.27.3 |
| [jwter](jwter/README.md) | Inspect compact JWTs; sign and verify HS256/ES256 tokens | 1.27.3 |
| [timetool](timetool/README.md) | Convert Unix seconds/milliseconds, RFC3339, and time zones | 1.27.3 |
| [portwho](portwho/README.md) | Inspect local TCP listeners and observed process owners; release verification in progress | 1.27.3 |

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

## Dependency security CI

[Dependency security](.github/workflows/security.yml) scans all seven modules on
pull requests, pushes to `main`, daily at 22:23 UTC (07:23 KST), and manual runs.
Each module runs independently on Linux with `CGO_ENABLED=0`, a current stable Go
toolchain, and the pinned `govulncheck` version documented above.

The scan includes tests and fails when vulnerable functions are reachable.
Module- or package-level findings without a reported call path remain visible in
the verbose logs. Plain-text output preserves the scanner's failure exit status;
JSON/SARIF output alone must not be used as the pass/fail gate.

Only scheduled runs on `main` create or update a security issue when the scanner
confirms reachable vulnerabilities (exit code 3). PRs, pushes, manual runs, and
installation, network or analysis errors alone do not create issue notifications.
The report includes affected modules, the scanned commit, and links to the
vulnerability details and call paths in the logs. If another module fails to
scan, the report marks that result as unknown.

One open issue is reused, with its body updated to the latest findings. Close it
after remediation; a later finding creates a new issue and preserves the closed
one. A separate comment mentions `@w00j00ng` once per scheduled-run date in KST
for each issue, including on creation. Rerunning that day's scan updates the body
without adding another notification comment. Issue reporting is serialized to
avoid duplicate issues from concurrent runs. Successful scans do not modify or
automatically close an existing issue.

Only the reporting job receives `issues: write` and `actions: read`; it uses the
built-in `GITHUB_TOKEN`, so no personal token or mail-service secret is needed.
Email delivery uses GitHub's participating/mention and comment email settings;
Actions notification emails can remain disabled. Keep the issue marker in the
body so subsequent scans can find the open issue.

Test issue creation, reuse and notification rules locally without accessing
GitHub or sending email:

```sh
python3 -B -m unittest discover -s .github/scripts -p 'test_security_issue.py' -v
```

A successful scan does not cover Windows/macOS-specific or CGO-enabled code, and
it is separate from Dependabot alerts and automatic dependency-update pull
requests. The workflow does not enable or change those repository settings.
