# JWT Tool

Inspect, verify, and sign compact JWTs. Signing and verification support HS256
and ES256. All processing is local.

## Install

Requires Go 1.21 or later. From this directory:

```sh
go install .
```

## Inspect or verify a token

```powershell
jwttool decode "$token"
jwttool decode "$token" --alg HS256 --secret-env JWT_SECRET
jwttool decode "$token" --alg ES256 --key-file .\public.pem
jwttool decode --token "$token"
Get-Content -Raw .\token.txt | jwttool decode --token-file -
```

Use exactly one positional token, `--token`, or `--token-file`. Combining input
sources or providing multiple positional tokens is an error, even when the
values are identical. The file name `-` reads stdin.
Outer whitespace is removed. Omitting `--alg` reads the header's `alg`; specifying
it requires a match. No algorithms are tried in sequence.

Every input form returns the original header, body in `payload`, and validation
status. The header includes `alg` and `typ` when present; absent fields are not
added. Without a key, the command only inspects the token, including tokens using
other algorithms or `none`, and explicitly reports that validation was not checked:

```json
{
  "header": {"alg": "HS256", "typ": "JWT"},
  "payload": {"sub": "user-123"},
  "validation": {
    "signature": "not_checked",
    "claims": "not_checked",
    "valid": null,
    "errors": []
  }
}
```

Provide `--secret` or `--secret-env` to verify HS256, or `--key-file` with a P-256
public key to verify ES256. These inputs are mutually exclusive. Even without
`--alg`, the key type fixes the permitted algorithm. An invalid key never falls
back to inspection.

Verification checks the signature first, then `exp` and `nbf` when present.
Expiration at the current time fails; `nbf` at the current time passes. There is
no clock leeway. Missing time claims are allowed, invalid numeric types fail,
and `iat` is displayed without checking its age. `valid` becomes `true` only when
these checks pass. Signature or claim failures still return the header and body
with `valid: false`, the reason in `errors`, and exit code 1. Claims stay
`not_checked` when the signature fails.

This local result does not check issuer/audience policy, revocation, account
status, or application permissions.

## Sign a token

```powershell
jwttool encode --body-file .\claims.json --alg HS256 --secret-env JWT_SECRET
jwttool encode --body '{"sub":"user-123"}' --alg ES256 --key-file .\private.pem
Get-Content -Raw .\claims.json | jwttool encode --body-file - --alg HS256 --secret-env JWT_SECRET
```

Use exactly one of `--body` and `--body-file`, and explicitly select `--alg HS256`
or `--alg ES256`. The body must be one UTF-8 JSON object. Claims, nested values,
and large integers are preserved; no expiration or other claim is added.
The output is one JWT line with `typ: JWT` and the selected algorithm.

HS256 requires a nonempty secret from `--secret` or the named environment
variable. The secret is used as-is, without trimming or implicit base64 decoding.
Prefer an environment variable to putting secrets in shell history or process
arguments. Supply a cryptographically random secret with sufficient entropy.

ES256 requires a P-256 private key in unencrypted SEC1 (`EC PRIVATE KEY`) or
PKCS#8 (`PRIVATE KEY`) PEM format. Verification accepts a PKIX (`PUBLIC KEY`)
P-256 public key. Other curves, certificates, encrypted private keys, and extra
PEM blocks are rejected. JWT signing protects integrity; it does not encrypt
the body.

## Input and exit codes

Input is explicit through the positional token, flags, files, stdin, and environment variables; there
are no interactive prompts. Token/body input is limited to 1 MiB and key input
to 64 KiB. Malformed base64url/JSON, duplicate header or top-level claim names, and unsupported
critical JOSE extensions are rejected. JWE and nested JWTs are not supported.

Results go to stdout; input/runtime errors go to stderr. Errors do not echo
token contents, secrets, or key material.

| Exit code | Meaning |
| --- | --- |
| 0 | Inspection/signing succeeded, or requested verification passed |
| 1 | Token signature, allowed algorithm, or time-claim verification failed |
| 2 | Invalid input, malformed token, key error, or execution error |

Inspection without a key returns exit 0 and shows `valid: null`, not `true`.
An explicit unsupported `--alg` or a key/algorithm input mismatch is an input
error. A token header incompatible with the supplied verification key fails
verification.

## Development

Algorithm implementations live in `algorithm/hs256` and `algorithm/es256`.
The common interface and enabled list in `token/algorithms.go` drive signing,
verification, CLI help, and supported-algorithm errors. See
[algorithm structure and extension review](ALGORITHMS.md) for the contract,
add/remove steps, and other algorithm candidates.

Run from this module directory:

```sh
go test -mod=readonly ./... -count=1
go vet -mod=readonly ./...
go build -mod=readonly .
```

Tests cover published RFC 7515 vectors, both signing algorithms, time boundaries,
tampering, invalid keys, JSON precision, input sources, and actual CLI exit codes.
The published example keys in `testdata` are for tests only.

References: [JWT](https://www.rfc-editor.org/rfc/rfc7519.html),
[JWT best practices](https://www.rfc-editor.org/rfc/rfc8725.html), and
[golang-jwt](https://github.com/golang-jwt/jwt).
