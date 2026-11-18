# tdelta

Compare UTF-8 text files and save a compressed delta. Apply the saved delta to
the original bytes to generate the target, including its whitespace and line endings.

From this directory, build with `go build .` or install with `go install .`.

~~~sh
tdelta diff A.txt B.txt
tdelta diff A.txt B.txt --color=never
tdelta create A.txt B.txt --output change.dcd
tdelta apply A.txt change.dcd --output B.generated.txt
tdelta show A.txt change.dcd
tdelta show A.txt change.dcd --editor
~~~

`diff` shows a unified diff with three context lines. `show` reconstructs and
verifies the target first, then displays the changes. Their exit codes are
`0` for identical content, `1` for differences, and `2` for errors.
`create` and `apply` return `0` on success and `2` on error. One input
may be `-` for stdin. `apply --output -` writes the verified target bytes to
stdout. File outputs must be new files. The output filesystem must support
hard links so the finished file can be published without overwriting another
file. An I/O failure on stdout may leave bytes already delivered to the caller.

In auto mode output has color on supported terminals, except with nonempty
`NO_COLOR` or `TERM=dumb`. `--color=always` forces ANSI color on stdout.
The display escapes control characters such as ESC and CR. Inputs are compared
and reconstructed byte for byte, including CRLF, BOM and missing final newlines.
Invalid UTF-8 and NUL are rejected.

## Editor

Use `--editor` on `diff` or `show`. Editor selection follows
`--editor-command`, `TDELTA_EDITOR`, `VISUAL`, then `EDITOR`.

~~~sh
tdelta diff A.txt B.txt --editor --editor-command "code --wait"
~~~

The command must wait until the temporary diff is closed. The editor receives
a plain, uncolored `change.diff`. Edits to it are discarded when the command
exits. On Windows the editor starts in the temporary directory on a local
drive, so use absolute paths for explicit executable paths or options that
refer to other files. The editor itself can also be found via `PATH`.

## Delta format and limits

The v1 `.dcd` file starts with `TDLT`, version byte 1, unsigned varint
source and target byte counts, and their raw SHA-256 digests. One zlib stream
contains byte-range COPY commands and literal INSERT commands. This format
requires the exact A to retrieve B. There is no reverse apply to recover A from B.

Each text can be up to 8 MiB. The delta input limit is 16 MiB and its decoded
command limit is 32 MiB. After matching leading and trailing lines are trimmed,
if both remaining sides are nonempty, Myers compares at most 1,000 lines in
total. Above this limit, `create` stores the entire target as one compressed
INSERT, `apply` can use that delta, and `diff` and `show` return an error
with no comparison output. A small delta can be larger than the target.

## Verification

~~~sh
go test -mod=readonly -count=1 -timeout=2m ./...
go vet -mod=readonly ./...
go build -mod=readonly .
go test -mod=readonly -run '^$' -fuzz '^FuzzApply$' -fuzztime=30s ./internal/delta
go test -mod=readonly -run '^$' -bench . -benchmem ./internal/delta
~~~

The CLI and regression suite were run on Windows amd64 and Ubuntu 26.04 in
WSL (Linux amd64) with Go 1.27.4, using a batch file and a shell script as
editor test doubles. Real GUI and terminal editor sessions have not been
verified. macOS validation is limited to cross-builds.

In three single-iteration benchmark runs on Windows amd64 (Core i7-13700K),
an 8 MiB input with one byte changed produced a 108-byte delta. Creating that
delta took 11–12 ms and applying it took 9–10 ms. A complete replacement of
the synthetic 8 MiB input produced an 8,389,343-byte delta. The 1,000-line
case with long shared prefixes took 144–169 ms to create the delta and
allocated about 32 MiB per operation. Allocation totals are not peak memory.
