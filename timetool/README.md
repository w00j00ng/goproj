# Time Tool

Convert current or supplied times between Unix seconds, Unix milliseconds, and
RFC3339. All conversions run locally.

## Install

Requires Go 1.27.3 or newer. From this checkout, run inside the timetool directory:

```sh
go install .
```

After publication, the module can also be installed with:

```sh
go install github.com/w00j00ng/goproj/timetool@latest
```

Remote @latest does not include these changes until they are published. The
minimum Go version includes the standard-library security fixes verified for
this tool. With the default GOTOOLCHAIN=auto, Go selects a suitable toolchain;
GOTOOLCHAIN=local rejects an older installation. Keep the build toolchain patched
for subsequent security fixes.

## Current time

```sh
timetool
timetool unix
timetool ms
timetool rfc3339 --tz Asia/Seoul
```

Running without a subcommand prints the current time as UTC RFC3339. The unix and
ms commands print the current Unix seconds and milliseconds. Each invocation
samples the clock once.

## Convert an input time

```sh
# RFC3339 to Unix seconds
timetool unix 2000-01-01T09:00:00+09:00
# 946684800

# RFC3339 to Unix milliseconds
timetool ms 2000-01-01T00:00:00.123Z
# 946684800123

# Unix seconds to RFC3339
timetool rfc3339 0 --from unix
# 1970-01-01T00:00:00Z

# Unix milliseconds to RFC3339 in Seoul
timetool rfc3339 123 --from ms --tz Asia/Seoul
# 1970-01-01T09:00:00.123+09:00

# RFC3339 time zone conversion
timetool rfc3339 2000-01-01T00:00:00Z --tz Asia/Seoul
# 2000-01-01T09:00:00+09:00

# Unix seconds to milliseconds
timetool ms 1 --from unix
# 1000

# Unix milliseconds to seconds
timetool unix 1001 --from ms
# 1

# A negative input goes after --, with all options before it
timetool unix --from ms -- -1
# -1
```

## Commands and options

```text
timetool
timetool unix [value] [--from rfc3339|unix|ms]
timetool ms [value] [--from rfc3339|unix|ms]
timetool rfc3339 [value] [--from rfc3339|unix|ms] [--tz name]
timetool help [unix|ms|rfc3339]
```

| Option | Default | Meaning |
| --- | --- | --- |
| --from | rfc3339 | Input format: rfc3339, unix (seconds), or ms (milliseconds) |
| --tz | UTC | RFC3339 output time zone: UTC, an IANA name such as Asia/Seoul or America/New_York, or Local |

Options work before or after the input value. Only one value is accepted.
An omitted value uses the current time, except that explicitly setting --from
requires a value. Empty input is an error. Numeric units are never inferred.

--tz belongs only to rfc3339; unix and ms reject it. It changes the displayed
time without changing the instant. The input offset determines the input
instant; --tz does not interpret an input that lacks an offset. Local explicitly
uses the machine's configured time zone.

Use timetool help, timetool help rfc3339, or timetool rfc3339 --help for help.

## Precision and limits

RFC3339 inputs require a full date and time, uppercase T, and either Z or a
numeric offset in the form +09:00. Fractions use a decimal point and one to nine
digits. Invalid dates, leap seconds, comma fractions, missing offsets, and
fractions beyond nanosecond precision are rejected. Date-only, natural-language,
and custom date formats are unsupported.

RFC3339 output preserves nanoseconds, removing unnecessary trailing fractional
zeros. Unix outputs are integers rounded down to the selected unit, including
before the epoch: 1969-12-31T23:59:59.999999999Z becomes -1 in both seconds and
milliseconds. Conversion uses integer arithmetic.

Inputs must represent a UTC year in 0000..9999. Accepted Unix ranges are:

| Unit | Minimum | Maximum |
| --- | --- | --- |
| Seconds | -62167219200 | 253402300799 |
| Milliseconds | -62167219200000 | 253402300799999 |

RFC3339 output also requires the selected zone's local year to be in 0000..9999.
Historical zone offsets containing seconds cannot be represented exactly in
RFC3339 and return an explicit error. UTC output is available for those instants.

The binary embeds Go's time zone database as a fallback when system data is
unavailable, including Windows machines without a Go installation. Time zone
rules depend on the available database; rebuild with an updated toolchain to
refresh the embedded data.

## Output and exit codes

Successful conversion writes one result followed by a newline to stdout.
Help also uses stdout. Errors use stderr and never print a conversion result.
A failed stdout write returns an error; already written bytes cannot be recalled.

| Exit code | Meaning |
| --- | --- |
| 0 | Conversion or help succeeded |
| 2 | Invalid input, unsupported options, time zone error, or output error |

## Development

Run from this module's directory:

```sh
go test -mod=readonly -count=1 -timeout=2m ./...
go vet -mod=readonly ./...
go build -mod=readonly -o /path/to/temporary/timetool .
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
```

Tests call the command with explicit requests and expected results, and also
build and run the actual CLI. They cover all input/output format combinations,
current-time bounds, time zones and DST, precision and negative values, invalid
inputs, help, exit codes, and output errors.
