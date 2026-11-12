# goproj

Small, independent Go command-line tools for development. Each directory has
its own `go.mod`; install and validate each module separately.

| Module | Purpose | Minimum Go |
| --- | --- | --- |
| [bcrypter](bcrypter/README.md) | Generate and verify bcrypt password hashes | 1.27.4 |
| [scrypter](scrypter/README.md) | Generate and verify Better Auth default scrypt hashes within documented Unicode limits | 1.27.4 |
| [helloworld](helloworld/README.md) | Inspect HTTP requests with local defaults and bounded, opt-in body logging | 1.27.4 |
| [jumin](jumin/README.md) | Generate synthetic resident-number-shaped test values | 1.27.4 |
| [jwter](jwter/README.md) | Inspect compact JWTs; sign and verify HS256/ES256 tokens | 1.27.4 |
| [timetool](timetool/README.md) | Convert Unix seconds/milliseconds, RFC3339, and time zones | 1.27.4 |
| [portwho](portwho/README.md) | Inspect local TCP listeners and observed process owners; release verification in progress | 1.27.4 |

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
Each module runs independently in four native runner configurations: Linux with
`CGO_ENABLED=0` and `1`, Windows with `CGO_ENABLED=0`, and macOS with
`CGO_ENABLED=1`. The matrix has 28 scan jobs. Linux CGO scans install X11 headers;
macOS uses the runner's compiler and SDK. Each result records the actual OS,
architecture, CGO setting, Go version, and pinned scanner version. Other
architectures are outside this matrix.

The scan includes tests and fails when vulnerable functions are reachable.
Package- and module-level findings without a reported call path pass the CI gate
and remain dependency-update warnings. The shared Task classifies the scanner's
JSON findings explicitly: JSON output can exit successfully even when it contains
reachable vulnerabilities. Scanner, JSON, and result-transfer errors fail the
check and are not treated as clean scans.

Only scheduled runs on `main` create or update a security issue for reachable
vulnerabilities or dependency-update warnings. The report runs after successful
scans as well as failed scans, so warning-only results are included. PRs, pushes,
manual runs, and installation, network or analysis errors alone do not create
issue notifications. One issue groups the findings by evidence level and includes
affected modules, platforms, found/fixed versions, the scanned commit, and links
to the vulnerability details and call paths. If another scan fails, its result is
unknown.

One open issue is reused, with its body updated to the latest findings. Close it
after remediation; a later finding creates a new issue and preserves the closed
one. A separate comment mentions `@w00j00ng` once per scheduled-run date in KST
for each issue, including on creation. Rerunning that day's scan updates the body
without adding another notification comment. Go scan jobs upload
`security-result.json` artifacts, with a unique name per module, configuration,
and scan attempt. Report-only retries reuse each configuration's last scan result
within the same workflow run. A newer scan with a missing, expired, or invalid
artifact is unknown; the report does not fall back to an older scan. Run and
attempt markers
prevent older reports from overwriting newer issue content or adding stale
comments. Issue reporting is serialized with `queue: max` to preserve up to 100
pending reports and avoid duplicate issues from concurrent runs. Unresolved
findings confirmed by the next scheduled scan update the open
issue and produce that date's mention comment. After closure, a new scheduled
run creates a fresh issue even at the same commit if findings persist. Each body
records the commit SHA and workflow run ID in a `dependency-security-source`
marker. Retries with the same SHA and run ID reuse the scan identity; a closed
issue suppresses that replay. Completion and closure timestamps do not determine
whether findings are reported. Existing issues use ordering markers or their
recorded SHA and run link for compatibility. Dates deduplicate mention comments.
Clean scans do not modify or automatically close an existing issue.

The shared report task checks the issue state returned by an update. If the
issue closed before a new scan reached it, the task reports that scan in a new
issue. A temporary hidden marker retains the previous report for recovery after
API failures. An update acknowledged while open counts as reported if the user
then closes the issue. Closure during an update can leave recovery metadata in
the closed issue.

Only the reporting job receives `issues: write` and `actions: read`; it uses the
built-in `GITHUB_TOKEN`, so no personal token or mail-service secret is needed.
Email delivery uses GitHub's participating/mention and comment email settings;
Actions notification emails can remain disabled. Keep the issue marker in the
body so subsequent scans can find the open issue.

The root [Taskfile.yml](Taskfile.yml) includes the public
[w00j00ng/tasks](https://github.com/w00j00ng/tasks) repository as the `tasks` Git
submodule for scanning, issue reporting and behavior tests. CI pins Task v3.54.0. Install that version and `govulncheck` for
local use; reporting also requires GitHub CLI (`gh`), `jq`, and the GitHub Actions
environment variables. The behavior tests use `jq` and POSIX command tools,
replace `gh` with a local response stub, and never access GitHub or send email.
Python is not required.

```sh
git submodule update --init --recursive
go install github.com/go-task/task/v3/cmd/task@v3.54.0
go install golang.org/x/vuln/cmd/govulncheck@v1.9.0
task security:test
task --exit-code security:scan MODULE=helloworld
```

Run these tasks from the repository root, with the Go bin directory on `PATH`.
`security:scan` accepts one of the seven module names. Locally it returns 3 for
reachable vulnerabilities, 0 for warnings or no findings, and a nonzero error for
failed scans. It preserves an explicit `CGO_ENABLED` setting and defaults to 0.
In CI it records reachable findings in `GITHUB_OUTPUT`, and the workflow's
following step fails the job for those findings. `SECURITY_RESULT_FILE` selects
the result file and `SECURITY_SCAN_KEY` identifies the module/configuration.
`security:report`
handles the scheduled-main issue and comment rules described above. Keep GitHub
credentials in `GH_TOKEN`; do not put them in Taskfile variables or commands.

These scans cover only the recorded native runner configurations. Static
analysis can miss calls, and it is separate from Dependabot alerts and automatic
dependency-update pull requests. The workflow does not enable or change those
repository settings.

The submodule is pinned to a reviewed commit. CI checks it out with
`submodules: true`; updates to the common repository take effect only after the
submodule pointer is updated here and verified. For a fresh clone, use
`git clone --recurse-submodules https://github.com/w00j00ng/goproj.git`.
