# Bcrypter

## Install

```sh
$ go install github.com/w00j00ng/goproj/bcrypter@latest
```

## Run

1.1. Enter Password

```sh
$ bcrypter
Enter Password: 
```

1.2. Check bcrypted text

```sh
$ bcrypter
$2a$10$V3nzSqajt.5FDZ/b/wW88eBpeMMYUr8f8swjjVw7pMOghGTrRlxQG
```

2.1. If you don't want to print the bcrypted text on the console then you should run it like below

```sh
> bcrypter --clipboard=true
Enter Password: 
```

2.2. Check Password

```sh
$ bcrypter --clipboard=true
copied on the clipboard!
```

## Verify a password

```sh
bcrypter verify --hash '<bcrypt-hash>'
Enter Password:
Password is valid.
```

Omit `--hash` to enter the hash at a prompt. Password prompts do not echo input.
For scripted checks, use `--password` (or `-p`):

```sh
bcrypter verify --hash '<bcrypt-hash>' --password 'example-password'
```

Quote hashes with single quotes in shells such as PowerShell to preserve `$`.
Prefer the password prompt for sensitive passwords: command-line passwords may
appear in shell history or process listings. Explicit `--password=''` verifies
an empty password; omitting the flag requests a prompt. Passwords are compared
without trimming or Unicode normalization. bcrypt passwords are limited to
72 bytes, including multi-byte UTF-8 characters.

Verification writes `Password is valid.` or `Password does not match.` to stdout.
Malformed hashes and input errors are reported on stderr.

| Verify exit code | Meaning |
| --- | --- |
| `0` | Password matches |
| `1` | Password does not match |
| `2` | Invalid input, malformed hash, or execution error |

## Development

Run from this module's directory:

```sh
go test ./...
go vet ./...
go build .
```

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

## Generation output and accepted hash formats

Generation prints only the hash followed by a newline to stdout. The password
prompt, clipboard confirmation and errors use stderr. Generation exits 0 on
success and 2 on input, hashing or clipboard errors. Verification retains its
0/1/2 success/mismatch/error contract.

Verification supports only bcrypt prefixes `$2a$`, `$2b$` and `$2y$`.
Legacy `$2$`, `$2x$`, uppercase and unknown minor prefixes are rejected as
invalid input. The unused `--toggle` option has been removed.
