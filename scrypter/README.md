# Scrypter

Generate and verify password hashes for Better Auth's default scrypt format,
within the Unicode compatibility limits below.

## Install from this checkout

Run from the `scrypter` directory:

```sh
go install .
```

## Generate a hash

```sh
scrypter
Enter Password:
<salt-hex>:<key-hex>
```

The password prompt does not echo input. Each invocation generates a fresh
random salt. Use `scrypter --clipboard` to copy the hash to the clipboard
instead of printing it.

## Verify a password

```sh
scrypter verify --hash '<salt-hex>:<key-hex>'
Enter Password:
Password is valid.
```

Omitting `--hash` prompts for the hash too. For scripted checks, use `--password`
(or `-p`):

```sh
scrypter verify --hash '<salt-hex>:<key-hex>' --password 'example-password'
```

Prefer the password prompt for sensitive passwords: command-line passwords may
appear in shell history or process listings. Explicit `--password=''` verifies
an empty password; omitting the flag requests a prompt.

Verification writes `Password is valid.` or `Password does not match.` to stdout.
Malformed hashes and input errors are reported on stderr.

| Exit code | Meaning |
| --- | --- |
| `0` | Successful generation or matching password |
| `1` | Password does not match |
| `2` | Invalid input, malformed hash, or execution error |

## Better Auth compatibility

The supported format is exactly 32 hex salt characters, a colon, and 128 hex
key characters. It does not embed parameters or a version.

| Setting | Value |
| --- | --- |
| `N` | `16384` |
| `r` | `16` |
| `p` | `1` |
| Derived key | 64 bytes, encoded as hex |
| Salt generation | 16 random bytes, encoded as 32 hex characters |
| Salt passed to scrypt | The 32-character hex string as UTF-8 text |
| Password preprocessing | Unicode NFKC normalization, then UTF-8 encoding |

Inputs must be valid UTF-8. Go applies stream-safe NFKC, unlike JavaScript:
when normalization would insert U+034F (COMBINING GRAPHEME JOINER), both
generation and verification return an explicit error (CLI exit 2), not a
password mismatch. For example, `"a" + "\u0301".repeat(40)` is unsupported.
The limit depends on decomposed non-starters, not simply string length.
U+034F supplied in the original password is preserved. This is a deliberate
support limit; full JavaScript normalization for these inputs is not implemented.
Unicode results also depend on the tables in Go's x/text and the JavaScript
runtime; this does not promise compatibility with every runtime/Unicode version.

Whitespace is preserved. Verification compares derived keys in constant time.
Custom Better Auth password functions and other scrypt profiles are unsupported.
This CLI does not enforce Better Auth's application-level password length policy.

References: [Better Auth password implementation](https://github.com/better-auth/utils/blob/main/src/password.node.ts),
[Better Auth v1.3.34](https://github.com/better-auth/better-auth/blob/v1.3.34/packages/better-auth/src/crypto/password.ts).

## Development

Run from this module's directory:

```sh
go test ./...
go vet ./...
go build .
```

The tests cover fixed Node.js scrypt fixtures, Unicode normalization, random
salts, malformed hashes, prompts, and CLI output and exit codes.

Unicode fixture regeneration: `node password/testdata/generate.mjs`.
The JSON records Node.js, ICU and Unicode versions; normal tests need only Go.

## Clipboard support

`--clipboard` is optional. Windows uses its native clipboard API and does not
require CGO. Linux requires CGO, a C compiler, X11 development headers (such as
`libx11-dev` on Debian/Ubuntu), and an accessible X11 display. macOS requires
CGO and its platform compiler/frameworks.

With `CGO_ENABLED=0` on Linux/macOS, hashing and verification still work;
`--clipboard` returns an explicit error instead of panicking. Other operating
systems do not have a clipboard implementation in this tool. Clipboard
initialization/write failures return an error and never report a successful copy.
Clipboard persistence after the program exits depends on the desktop environment
and clipboard manager, especially on X11.

Building this module requires Go 1.27.3 or newer.
