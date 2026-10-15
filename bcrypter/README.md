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
