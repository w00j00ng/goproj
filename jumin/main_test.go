package main

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func TestCLI(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "jumin")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	if output, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	for _, tt := range []struct {
		name   string
		args   []string
		code   int
		prefix string
		stderr string
	}{
		{"missing", nil, 2, "", "exactly one"},
		{"bad date", []string{"bad-date"}, 2, "", "YYYY-MM-DD"},
		{"invalid leap day", []string{"2001-02-29"}, 2, "", "YYYY-MM-DD"},
		{"invalid gender", []string{"--gender=invalid", "2001-01-01"}, 2, "", "gender"},
		{"empty gender", []string{"--gender=", "2001-01-01"}, 2, "", "gender"},
		{"date then option", []string{"2001-01-01", "--gender=female"}, 2, "", "exactly one"},
		{"extra date", []string{"2001-01-01", "2002-01-01"}, 2, "", "exactly one"},
		{"too early", []string{"1899-12-31"}, 2, "", "1900"},
		{"too late", []string{"2100-01-01"}, 2, "", "2099"},
		{"unknown flag", []string{"--unknown", "2001-01-01"}, 2, "", "flag provided"},
		{"help", []string{"--help"}, 0, "", "Usage: jumin"},
		{"default male", []string{"2001-01-01"}, 0, "010101-3", ""},
		{"1999 male", []string{"--gender=male", "1999-12-31"}, 0, "991231-1", ""},
		{"1999 female", []string{"--gender=female", "1999-12-31"}, 0, "991231-2", ""},
		{"2000 male", []string{"--gender=male", "2000-01-01"}, 0, "000101-3", ""},
		{"2000 female", []string{"--gender=female", "2000-01-01"}, 0, "000101-4", ""},
		{"lower boundary", []string{"1900-01-01"}, 0, "000101-1", ""},
		{"upper boundary", []string{"--gender=female", "2099-12-31"}, 0, "991231-4", ""},
		{"leap day", []string{"2000-02-29"}, 0, "000229-3", ""},
		{"male alias m", []string{"--gender=m", "2001-01-01"}, 0, "010101-3", ""},
		{"male alias man", []string{"--gender=MAN", "2001-01-01"}, 0, "010101-3", ""},
		{"female alias f", []string{"--gender=f", "2001-01-01"}, 0, "010101-4", ""},
		{"female alias woman", []string{"--gender=WOMAN", "2001-01-01"}, 0, "010101-4", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			command := exec.Command(binary, tt.args...)
			var stdout, stderr bytes.Buffer
			command.Stdout = &stdout
			command.Stderr = &stderr
			err := command.Run()
			code := 0
			if err != nil {
				exit, ok := err.(*exec.ExitError)
				if !ok {
					t.Fatal(err)
				}
				code = exit.ExitCode()
			}
			if code != tt.code || !strings.Contains(stderr.String(), tt.stderr) {
				t.Fatalf("exit=%d stdout=%q stderr=%q; want exit=%d stderr containing %q", code, stdout.String(), stderr.String(), tt.code, tt.stderr)
			}
			if tt.prefix == "" {
				if stdout.Len() != 0 {
					t.Fatalf("unexpected stdout=%q", stdout.String())
				}
				return
			}
			if stderr.Len() != 0 || !regexp.MustCompile("^[0-9]{6}-[0-9]{7}\n$").Match(stdout.Bytes()) || !strings.HasPrefix(stdout.String(), tt.prefix) {
				t.Fatalf("stdout=%q stderr=%q; want prefix=%s and 13 digits", stdout.String(), stderr.String(), tt.prefix)
			}
			digits := strings.ReplaceAll(strings.TrimSpace(stdout.String()), "-", "")
			sum := 0
			for i, weight := range []int{2, 3, 4, 5, 6, 7, 8, 9, 2, 3, 4, 5} {
				sum += int(digits[i]-'0') * weight
			}
			if int(digits[12]-'0') != (11-sum%11)%10 {
				t.Fatal("invalid check digit")
			}
		})
	}
}
