# Jumin

Generate a test value with a date prefix, gender digit, random digits and a
check digit. This is not a lookup or proof of a real, issued resident number.
Generated values can coincide with real numbers; use them only as synthetic
test data, never as evidence of identity.

## Install

```sh
go install github.com/w00j00ng/goproj/jumin@latest
```

## Run

```sh
jumin 1990-01-01
jumin --gender=female 2001-01-01
```

Exactly one valid date in `YYYY-MM-DD` format is required. Supported years are
1900–2099. The default gender is male. Accepted values are `male`, `m`, `man`,
`female`, `f`, `woman`, ignoring case.

Options must precede the date. For example, `jumin 2001-01-01 --gender=female`
is rejected; move the option before the date. Unknown options, extra arguments,
invalid dates, unsupported years and unknown genders are errors.

Success writes one `YYMMDD-XXXXXXX` line to stdout and exits 0. Errors go to
stderr with exit 2. `jumin --help` displays usage and exits 0.

## Development

```sh
go test -count=1 -timeout=2m ./...
go vet ./...
go build .
```
