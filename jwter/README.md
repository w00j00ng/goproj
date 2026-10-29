# jwter

Inspect, verify, and sign compact JWTs. Signing and verification support HS256
and ES256. All processing is local.

## Install

Requires Go 1.27.3 or later:

```sh
go install github.com/w00j00ng/goproj/jwter@latest
```

Or install the current checkout from this directory:

```sh
go install .
```

## Inspect or verify a token

```powershell
jwter decode "$token"
jwter decode "$token" --alg HS256 --secret-env JWT_SECRET
jwter decode "$token" --alg ES256 --key-file .\public.pem
jwter decode --token "$token"
Get-Content -Raw .\token.txt | jwter decode --token-file -
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

## Verify a token against a policy

```powershell
jwter verify "$token" --alg HS256 --secret-env JWT_SECRET --iss https://issuer.example --aud my-api --typ JWT --require exp
jwter verify "$token" --alg ES256 --key-file .\public.pem --sub user-123 --require exp,iat --max-age 15m --max-lifetime 1h
Get-Content -Raw .\token.txt | jwter verify --token-file - --alg HS256 --secret-env JWT_SECRET --leeway 30s
```

`verify` requires an explicit `--alg` and exactly one key input. It accepts the
same positional token, `--token`, and `--token-file` inputs as `decode`, with the
same duplicate-input and size checks. Signature verification runs before any
policy checks. A failed signature leaves all policy checks `not_checked`.

HS256 verification requires a secret of at least **32 bytes**, used as provided
without implicit base64 decoding. UTF-8 byte length, not character count, is
measured. Length does not prove entropy: use a cryptographically random key.
A matching signature made with a shorter key returns exit 1 with
`hmac_key_too_short`; `signature` remains `valid`, `key_length` is `invalid`, and
the overall `valid` is `false`. This key-length policy is specific to `verify`.
ES256 accepts the same P-256 public PEM keys described below.

| Option / field | Verification rule |
| --- | --- |
| `--iss`, `--sub`, `--jti` | Require a nonempty string matching the expected value exactly, with case-sensitive comparison |
| `--aud` | Require a nonempty string or nonempty array of nonempty strings containing the expected audience; all array members must have the correct type |
| `--typ` | Require the JOSE **header** media type; `JWT` and `application/jwt` are equivalent. Type/subtype and parameter names ignore case; parameter values compare exactly |
| `exp` | When present, require a NumericDate and `now < exp + leeway` |
| `nbf`, `iat` | When present, require NumericDates and `value <= now + leeway`; future `iat` rejection is a verification policy |
| `--require` | Require one or more of `iss,sub,aud,exp,nbf,iat,jti,typ`; repeat the option or use commas. Without an expected value, string fields are checked for presence and type only |
| `--leeway` | Nonnegative clock tolerance for `exp`, `nbf`, and `iat`; default `0s` |
| `--max-age` | Positive duration requiring `iat`; accept only when `now - iat <= limit` |
| `--max-lifetime` | Positive duration requiring `iat` and `exp`; accept only when `0 < exp - iat <= limit` |

RFC 2231 extended or continued media type parameter names containing `*` (such
as `profile*` or `profile*0`) are unsupported and rejected in both the token's
`typ` header and `--typ`. Ordinary parameter values may contain `*`, quoted
semicolons, and escaped quotes.

`--max-age` and `--max-lifetime` are disabled when omitted; explicitly specifying
zero or a negative value is an input error. Neither maximum is extended by
`--leeway`. NumericDate fractions and large numbers are compared exactly.
Optional missing fields are not failures. Expected-value options also make their
fields mandatory; explicitly empty expected values or an empty `--require` are
input errors.

Results always contain `header`, `payload`, and `validation`. In addition to the
aggregate status, `verify` returns `validation.checks` with `valid`, `invalid`, or
`not_checked` for each of `iss`, `sub`, `aud`, `exp`, `nbf`, `iat`, `jti`, `typ`,
`key_length`, `max_age`, and `max_lifetime`. Unconfigured string comparisons and
missing optional dates are `not_checked`. A required-only field marked `valid`
has passed presence/type checks, without checking an application-specific value.
If a maximum's required date is missing or malformed, the maximum is `invalid`
and `errors` identifies the missing/invalid date. The HS256 byte-length check is
`not_checked` for ES256.

Policy failures return the parsed header and payload, `valid: false`, stable
codes in `errors`, and exit 1. Examples include `iss_missing`, `aud_mismatch`,
`iat_invalid`, `token_issued_in_future`, `max_age_exceeded`,
`max_lifetime_exceeded`, and `max_lifetime_invalid` for a nonpositive interval.
Invalid options, malformed tokens, and key format/type errors return exit 2.

`valid: true` means the supplied key and selected local policy passed. The caller
must provide trusted expected values and the issuer's trusted key. A `jti` match
does not prove uniqueness or prevent replay, and a `sub` match does not query an
account database. No network requests, revocation checks, or permission checks
are performed. `decode` retains its optional signature and `exp`/`nbf` checks;
use `verify` for the additional policies above.

## Sign a token

```powershell
jwter encode --body-file .\claims.json --alg HS256 --secret-env JWT_SECRET
jwter encode --body '{"sub":"user-123"}' --alg ES256 --key-file .\private.pem
Get-Content -Raw .\claims.json | jwter encode --body-file - --alg HS256 --secret-env JWT_SECRET
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
tampering, invalid keys, JSON precision, input sources, verification policies,
HS256 byte-length limits, and actual CLI exit codes.
The published example keys in `testdata` are for tests only.

References: [JWT](https://www.rfc-editor.org/rfc/rfc7519.html),
[JWT best practices](https://www.rfc-editor.org/rfc/rfc8725.html), and
[golang-jwt](https://github.com/golang-jwt/jwt). HS256 key lengths follow
[RFC 7518 section 3.2](https://www.rfc-editor.org/rfc/rfc7518.html#section-3.2).
Header media types follow
[RFC 7515 section 4.1.9](https://www.rfc-editor.org/rfc/rfc7515.html#section-4.1.9).
