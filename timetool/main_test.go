package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRun(t *testing.T) {
	now := time.Date(2000, 1, 1, 0, 0, 0, 123456789, time.UTC)
	for _, tt := range []struct {
		name   string
		args   []string
		stdout string
		stderr string
		code   int
	}{
		{"default now", nil, "2000-01-01T00:00:00.123456789Z\n", "", 0},
		{"now seconds", []string{"unix"}, "946684800\n", "", 0},
		{"now milliseconds", []string{"ms"}, "946684800123\n", "", 0},
		{"now RFC3339", []string{"rfc3339"}, "2000-01-01T00:00:00.123456789Z\n", "", 0},
		{"now Seoul", []string{"rfc3339", "--tz", "Asia/Seoul"}, "2000-01-01T09:00:00.123456789+09:00\n", "", 0},
		{"RFC3339 to seconds", []string{"unix", "2000-01-01T09:00:00.123456789+09:00"}, "946684800\n", "", 0},
		{"RFC3339 to milliseconds", []string{"ms", "2000-01-01T09:00:00.123456789+09:00", "--from", "rfc3339"}, "946684800123\n", "", 0},
		{"RFC3339 to UTC", []string{"rfc3339", "2000-01-01T09:00:00.123456789+09:00"}, "2000-01-01T00:00:00.123456789Z\n", "", 0},
		{"seconds to seconds", []string{"unix", "946684800", "--from", "unix"}, "946684800\n", "", 0},
		{"seconds to milliseconds", []string{"ms", "946684800", "--from", "unix"}, "946684800000\n", "", 0},
		{"seconds to RFC3339", []string{"rfc3339", "946684800", "--from", "unix"}, "2000-01-01T00:00:00Z\n", "", 0},
		{"milliseconds to seconds", []string{"unix", "946684800123", "--from", "ms"}, "946684800\n", "", 0},
		{"milliseconds to milliseconds", []string{"ms", "946684800123", "--from", "ms"}, "946684800123\n", "", 0},
		{"milliseconds to RFC3339", []string{"rfc3339", "946684800123", "--from", "ms"}, "2000-01-01T00:00:00.123Z\n", "", 0},
		{"epoch", []string{"rfc3339", "0", "--from", "unix"}, "1970-01-01T00:00:00Z\n", "", 0},
		{"milliseconds to Seoul", []string{"rfc3339", "123", "--from", "ms", "--tz", "Asia/Seoul"}, "1970-01-01T09:00:00.123+09:00\n", "", 0},
		{"RFC3339 to Seoul", []string{"rfc3339", "2000-01-01T00:00:00Z", "--tz", "Asia/Seoul"}, "2000-01-01T09:00:00+09:00\n", "", 0},
		{"options before value", []string{"ms", "--from", "unix", "1"}, "1000\n", "", 0},
		{"options after value", []string{"ms", "1", "--from", "unix"}, "1000\n", "", 0},
		{"positive fractional floor", []string{"unix", "1001", "--from", "ms"}, "1\n", "", 0},
		{"negative milliseconds", []string{"rfc3339", "--from", "ms", "--", "-1"}, "1969-12-31T23:59:59.999Z\n", "", 0},
		{"negative floor", []string{"unix", "--from", "ms", "--", "-1"}, "-1\n", "", 0},
		{"negative seconds", []string{"ms", "--from", "unix", "--", "-1"}, "-1000\n", "", 0},
		{"negative fractional floor", []string{"ms", "1969-12-31T23:59:59.999999999Z"}, "-1\n", "", 0},
		{"just below first millisecond", []string{"ms", "1970-01-01T00:00:00.000999999Z"}, "0\n", "", 0},
		{"exact first millisecond", []string{"ms", "1970-01-01T00:00:00.001Z"}, "1\n", "", 0},
		{"just below negative millisecond", []string{"ms", "1969-12-31T23:59:59.998999999Z"}, "-2\n", "", 0},
		{"exact negative millisecond", []string{"ms", "1969-12-31T23:59:59.999Z"}, "-1\n", "", 0},
		{"fractional zeros removed", []string{"rfc3339", "2000-01-01T00:00:00.123000000Z"}, "2000-01-01T00:00:00.123Z\n", "", 0},
		{"DST before spring", []string{"rfc3339", "2024-03-10T06:59:59Z", "--tz", "America/New_York"}, "2024-03-10T01:59:59-05:00\n", "", 0},
		{"DST after spring", []string{"rfc3339", "2024-03-10T07:00:00Z", "--tz", "America/New_York"}, "2024-03-10T03:00:00-04:00\n", "", 0},
		{"DST first autumn hour", []string{"rfc3339", "2024-11-03T05:30:00Z", "--tz", "America/New_York"}, "2024-11-03T01:30:00-04:00\n", "", 0},
		{"DST second autumn hour", []string{"rfc3339", "2024-11-03T06:30:00Z", "--tz", "America/New_York"}, "2024-11-03T01:30:00-05:00\n", "", 0},
		{"minimum seconds", []string{"rfc3339", "--from", "unix", "--", "-62167219200"}, "0000-01-01T00:00:00Z\n", "", 0},
		{"minimum milliseconds", []string{"rfc3339", "--from", "ms", "--", "-62167219200000"}, "0000-01-01T00:00:00Z\n", "", 0},
		{"maximum seconds", []string{"ms", "253402300799", "--from", "unix"}, "253402300799000\n", "", 0},
		{"maximum milliseconds", []string{"rfc3339", "253402300799999", "--from", "ms"}, "9999-12-31T23:59:59.999Z\n", "", 0},
		{"maximum RFC3339", []string{"ms", "9999-12-31T23:59:59.999999999Z"}, "253402300799999\n", "", 0},
		{"minimum UTC through offset", []string{"unix", "0000-01-01T00:01:00+00:01"}, "-62167219200\n", "", 0},
		{"maximum UTC through offset", []string{"unix", "9999-12-31T23:58:59-00:01"}, "253402300799\n", "", 0},
		{"largest positive offset", []string{"unix", "2000-01-01T00:00:00+23:59"}, "946598460\n", "", 0},
		{"largest negative offset", []string{"unix", "2000-01-01T00:00:00-23:59"}, "946771140\n", "", 0},
		{"unknown command", []string{"unknown"}, "", "unknown", 2},
		{"extra value", []string{"unix", "2000-01-01T00:00:00Z", "extra"}, "", "at most", 2},
		{"empty input", []string{"unix", ""}, "", "RFC3339", 2},
		{"missing input", []string{"unix", "--from", "unix"}, "", "value", 2},
		{"explicit default without input", []string{"unix", "--from", "rfc3339"}, "", "value", 2},
		{"unknown from", []string{"unix", "0", "--from", "minutes"}, "", "--from", 2},
		{"empty from", []string{"unix", "0", "--from="}, "", "--from", 2},
		{"missing option value", []string{"unix", "--from"}, "", "flag needs an argument", 2},
		{"unknown option", []string{"unix", "--unknown"}, "", "unknown flag", 2},
		{"timezone on seconds", []string{"unix", "--tz", "UTC"}, "", "unknown flag", 2},
		{"timezone on milliseconds", []string{"ms", "--tz", "UTC"}, "", "unknown flag", 2},
		{"unknown timezone", []string{"rfc3339", "--tz", "Invalid/Zone"}, "", "time zone", 2},
		{"empty timezone", []string{"rfc3339", "--tz="}, "", "time zone", 2},
		{"timezone is not positional", []string{"rfc3339", "Asia/Seoul"}, "", "RFC3339", 2},
		{"no unit inference", []string{"unix", "1000"}, "", "RFC3339", 2},
		{"negative without separator", []string{"unix", "--from", "ms", "-1"}, "", "unknown shorthand flag", 2},
		{"decimal integer", []string{"unix", "1.5", "--from", "unix"}, "", "integer", 2},
		{"exponential integer", []string{"unix", "1e3", "--from", "ms"}, "", "integer", 2},
		{"integer overflow", []string{"unix", "9223372036854775808", "--from", "unix"}, "", "integer", 2},
		{"maximum int64 seconds", []string{"unix", "9223372036854775807", "--from", "unix"}, "", "range", 2},
		{"minimum int64 milliseconds", []string{"ms", "--from", "ms", "--", "-9223372036854775808"}, "", "range", 2},
		{"seconds below minimum", []string{"unix", "--from", "unix", "--", "-62167219201"}, "", "range", 2},
		{"seconds above maximum", []string{"unix", "253402300800", "--from", "unix"}, "", "range", 2},
		{"milliseconds below minimum", []string{"ms", "--from", "ms", "--", "-62167219200001"}, "", "range", 2},
		{"milliseconds above maximum", []string{"ms", "253402300800000", "--from", "ms"}, "", "range", 2},
		{"invalid leap day", []string{"unix", "2001-02-29T00:00:00Z"}, "", "RFC3339", 2},
		{"missing zone", []string{"unix", "2000-01-01T00:00:00"}, "", "RFC3339", 2},
		{"date only", []string{"unix", "2000-01-01"}, "", "RFC3339", 2},
		{"too much precision", []string{"unix", "2000-01-01T00:00:00.1234567890Z"}, "", "RFC3339", 2},
		{"comma fraction", []string{"unix", "2000-01-01T00:00:00,123Z"}, "", "RFC3339", 2},
		{"invalid offset hour", []string{"unix", "2000-01-01T00:00:00+24:00"}, "", "RFC3339", 2},
		{"invalid offset minute", []string{"unix", "2000-01-01T00:00:00+00:60"}, "", "RFC3339", 2},
		{"leap second", []string{"unix", "2000-01-01T00:00:60Z"}, "", "RFC3339", 2},
		{"UTC below minimum", []string{"unix", "0000-01-01T00:00:00+00:01"}, "", "range", 2},
		{"UTC above maximum", []string{"ms", "9999-12-31T23:59:59-00:01"}, "", "range", 2},
		{"output year overflow", []string{"rfc3339", "9999-12-31T23:59:59Z", "--tz", "Asia/Tokyo"}, "", "range", 2},
		{"output year underflow", []string{"rfc3339", "0000-01-01T00:00:00Z", "--tz", "Etc/GMT+1"}, "", "range", 2},
		{"historical seconds offset", []string{"rfc3339", "1900-01-01T00:00:00Z", "--tz", "Asia/Seoul"}, "", "offset", 2},
		{"unknown help topic", []string{"help", "missing"}, "", "unknown help topic", 2},
		{"extra help topic", []string{"help", "unix", "extra"}, "", "unknown help topic", 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr, now)
			if code != tt.code || stdout.String() != tt.stdout || !strings.Contains(stderr.String(), tt.stderr) || tt.stderr == "" && stderr.Len() != 0 {
				t.Fatalf("exit=%d stdout=%q stderr=%q; want exit=%d stdout=%q stderr containing %q", code, stdout.String(), stderr.String(), tt.code, tt.stdout, tt.stderr)
			}
		})
	}
}

func TestHelp(t *testing.T) {
	for _, tt := range []struct {
		args []string
		want string
	}{
		{[]string{"help"}, "rfc3339"},
		{[]string{"--help"}, "unix"},
		{[]string{"help", "unix"}, "--from"},
		{[]string{"rfc3339", "--help"}, "--tz"},
	} {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(tt.args, &stdout, &stderr, time.Time{}); code != 0 || !strings.Contains(stdout.String(), tt.want) || stderr.Len() != 0 {
				t.Fatalf("exit=%d stdout=%q stderr=%q; want help containing %q", code, stdout.String(), stderr.String(), tt.want)
			}
		})
	}
}

func TestRunUsesExplicitArguments(t *testing.T) {
	previous := os.Args
	os.Args = []string{"timetool", "ms", "0", "--from", "unix"}
	t.Cleanup(func() { os.Args = previous })
	var stdout, stderr bytes.Buffer
	now := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	if code := run(nil, &stdout, &stderr, now); code != 0 || stdout.String() != "2000-01-01T00:00:00Z\n" || stderr.Len() != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q; want explicit empty input to use current UTC time", code, stdout.String(), stderr.String())
	}
}

func TestLocalTimeZone(t *testing.T) {
	previous := time.Local
	time.Local = time.FixedZone("test-local", 5*3600+45*60)
	t.Cleanup(func() { time.Local = previous })
	now := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, tt := range []struct {
		args []string
		want string
	}{
		{[]string{"rfc3339", "2000-01-01T00:00:00Z", "--tz", "Local"}, "2000-01-01T05:45:00+05:45\n"},
		{[]string{"rfc3339", "--tz", "Local"}, "2000-01-01T05:45:00+05:45\n"},
		{nil, "2000-01-01T00:00:00Z\n"},
		{[]string{"unix"}, "946684800\n"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(tt.args, &stdout, &stderr, now); code != 0 || stdout.String() != tt.want || stderr.Len() != 0 {
			t.Fatalf("args=%q exit=%d stdout=%q stderr=%q; want %q", tt.args, code, stdout.String(), stderr.String(), tt.want)
		}
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("output unavailable") }

func TestOutputFailure(t *testing.T) {
	for _, args := range [][]string{{"unix"}, {"help"}} {
		var stderr bytes.Buffer
		if code := run(args, failingWriter{}, &stderr, time.Now()); code != 2 || !strings.Contains(stderr.String(), "output unavailable") {
			t.Fatalf("args=%q exit=%d stderr=%q; want output failure", args, code, stderr.String())
		}
	}
}

func TestCLI(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "timetool")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	if output, err := exec.Command("go", "build", "-mod=readonly", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}
	// Exercise the binary without the build machine's Go timezone files.
	t.Setenv("GOROOT", t.TempDir())
	t.Setenv("ZONEINFO", filepath.Join(t.TempDir(), "missing-zoneinfo"))
	for _, tt := range []struct {
		name   string
		args   []string
		stdout string
		stderr string
		code   int
	}{
		{"RFC3339 to seconds", []string{"unix", "2000-01-01T09:00:00+09:00"}, "946684800\n", "", 0},
		{"RFC3339 to milliseconds", []string{"ms", "2000-01-01T00:00:00.123Z"}, "946684800123\n", "", 0},
		{"seconds before flags", []string{"rfc3339", "0", "--from", "unix"}, "1970-01-01T00:00:00Z\n", "", 0},
		{"flags before value", []string{"ms", "--from", "unix", "1"}, "1000\n", "", 0},
		{"embedded time zone", []string{"rfc3339", "123", "--from", "ms", "--tz", "Asia/Seoul"}, "1970-01-01T09:00:00.123+09:00\n", "", 0},
		{"negative input", []string{"unix", "--from", "ms", "--", "-1"}, "-1\n", "", 0},
		{"unknown command", []string{"missing"}, "", "unknown", 2},
		{"input error", []string{"unix", "bad-time"}, "", "RFC3339", 2},
		{"missing value", []string{"ms", "--from", "unix"}, "", "value", 2},
		{"extra value", []string{"unix", "0", "extra", "--from", "unix"}, "", "at most", 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			command := exec.Command(binary, tt.args...)
			var stdout, stderr bytes.Buffer
			command.Stdout, command.Stderr = &stdout, &stderr
			err := command.Run()
			code := 0
			if err != nil {
				var exit *exec.ExitError
				if !errors.As(err, &exit) {
					t.Fatal(err)
				}
				code = exit.ExitCode()
			}
			if code != tt.code || stdout.String() != tt.stdout || !strings.Contains(stderr.String(), tt.stderr) || tt.stderr == "" && stderr.Len() != 0 {
				t.Fatalf("exit=%d stdout=%q stderr=%q; want exit=%d stdout=%q stderr containing %q", code, stdout.String(), stderr.String(), tt.code, tt.stdout, tt.stderr)
			}
		})
	}
	for _, args := range [][]string{nil, {"unix"}, {"ms"}, {"rfc3339"}} {
		t.Run("current "+strings.Join(args, " "), func(t *testing.T) {
			before := time.Now()
			command := exec.Command(binary, args...)
			var stdout, stderr bytes.Buffer
			command.Stdout, command.Stderr = &stdout, &stderr
			if err := command.Run(); err != nil || stderr.Len() != 0 {
				t.Fatalf("run: %v stderr=%q", err, stderr.String())
			}
			after := time.Now()
			value := strings.TrimSuffix(stdout.String(), "\n")
			if len(args) == 0 || args[0] == "rfc3339" {
				got, err := time.Parse(time.RFC3339Nano, value)
				if err != nil || got.Before(before) || got.After(after) || !strings.HasSuffix(value, "Z") {
					t.Fatalf("stdout=%q; want current UTC time in [%v, %v]", stdout.String(), before, after)
				}
				return
			}
			low, high := before.Unix(), after.Unix()
			if args[0] == "ms" {
				low, high = before.UnixMilli(), after.UnixMilli()
			}
			got, err := strconv.ParseInt(value, 10, 64)
			if err != nil || got < low || got > high {
				t.Fatalf("stdout=%q; want current %s in [%d, %d]", stdout.String(), args[0], low, high)
			}
		})
	}
	command := exec.Command(binary, "help", "rfc3339")
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil || !strings.Contains(stdout.String(), "--tz") || stderr.Len() != 0 {
		t.Fatalf("help: %v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
	}
}
