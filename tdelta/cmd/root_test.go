package cmd_test

import (
	"bytes"
	"compress/zlib"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/w00j00ng/goproj/tdelta/cmd"
)

func run(t *testing.T, input string, args ...string) (int, string, string) {
	t.Helper()
	var out, stderr bytes.Buffer
	code := cmd.Run(args, strings.NewReader(input), &out, &stderr)
	return code, out.String(), stderr.String()
}

func put(t *testing.T, path, data string) string {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCLIComparisonLimit(t *testing.T) {
	d := t.TempDir()
	a := put(t, filepath.Join(d, "a"), strings.Repeat("a\n", 500))
	at := put(t, filepath.Join(d, "at"), strings.Repeat("b\n", 500))
	bText := strings.Repeat("b\n", 501)
	b := put(t, filepath.Join(d, "b"), bText)
	p := filepath.Join(d, "change.dcd")
	if code, out, err := run(t, "", "diff", a, at); code != 1 || !strings.Contains(out, "@@ -1,500 +1,500 @@") || err != "" {
		t.Fatalf("at limit: %d %q %q", code, out, err)
	}
	if code, out, err := run(t, "", "create", a, b, "--output", p); code != 0 || out != "" || err != "" {
		t.Fatalf("create: %d %q %q", code, out, err)
	}
	if code, out, err := run(t, "", "apply", a, p, "--output", "-"); code != 0 || out != bText || err != "" {
		t.Fatalf("apply: %d %q", code, err)
	}
	for _, args := range [][]string{{"diff", a, b}, {"show", a, p}, {"show", a, p, "--editor", "--editor-command", "nonexistent-editor"}} {
		if code, out, err := run(t, "", args...); code != 2 || out != "" || !strings.Contains(err, "1001") || !strings.Contains(err, "1000") {
			t.Fatalf("%q: %d %q %q", args, code, out, err)
		}
	}
}

func TestCLIHunks(t *testing.T) {
	var a, b, shift strings.Builder
	for i := 1; i <= 20; i++ {
		line := fmt.Sprintf("line %02d\n", i)
		a.WriteString(line)
		if i == 2 || i == 4 || i == 15 {
			line = "new " + line
		}
		b.WriteString(line)
		shift.WriteString(line)
		if i == 2 {
			shift.WriteString("insert here\n")
		}
	}
	for _, tc := range []struct {
		name, a, b string
		want       []string
		code       int
	}{
		{"multiple hunks", a.String(), b.String(), []string{"@@ -1,7 +1,7 @@", "@@ -12,7 +12,7 @@"}, 1},
		{"shifted", a.String(), shift.String(), []string{"@@ -1,7 +1,8 @@", "@@ -12,7 +13,7 @@"}, 1},
		{"empty before", "", "added\n", []string{"@@ -0,0 +1 @@"}, 1},
		{"empty after", "removed\n", "", []string{"@@ -1 +0,0 @@"}, 1},
		{"missing newline", "x\n", "x", []string{"@@ -1 +1 @@"}, 1},
		{"same", "a", "a", nil, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bFile := put(t, filepath.Join(t.TempDir(), "b.txt"), tc.b)
			code, out, stderr := run(t, tc.a, "diff", "-", bFile, "--color=never")
			var headers []string
			for _, line := range strings.Split(out, "\n") {
				if strings.HasPrefix(line, "@@") {
					headers = append(headers, line)
				}
			}
			if code != tc.code || stderr != "" || strings.Join(headers, "\n") != strings.Join(tc.want, "\n") {
				t.Fatalf("diff = %d, %q, %q; headers want %q", code, out, stderr, tc.want)
			}
			if tc.name == "missing newline" && !strings.Contains(out, "+x\n\\ No newline at end of file\n") {
				t.Fatalf("missing EOF marker in %q", out)
			}
		})
	}
}

func TestCLINUL(t *testing.T) {
	for _, tc := range []struct{ name, base, target string }{{"base", "a\x00", "B"}, {"target", "A", "b\x00"}} {
		t.Run(tc.name, func(t *testing.T) {
			d := t.TempDir()
			a := put(t, filepath.Join(d, "a"), tc.base)
			output := filepath.Join(d, "out")
			var encoded bytes.Buffer
			encoded.Write([]byte{'T', 'D', 'L', 'T', 1, byte(len(tc.base)), byte(len(tc.target))})
			ba, bb := sha256.Sum256([]byte(tc.base)), sha256.Sum256([]byte(tc.target))
			encoded.Write(ba[:])
			encoded.Write(bb[:])
			z := zlib.NewWriter(&encoded)
			if _, err := z.Write(append([]byte{2, byte(len(tc.target))}, tc.target...)); err != nil {
				t.Fatal(err)
			}
			if err := z.Close(); err != nil {
				t.Fatal(err)
			}
			p := put(t, filepath.Join(d, "x.dcd"), encoded.String())
			for _, args := range [][]string{{"apply", a, p, "--output", output}, {"apply", a, p, "--output", "-"}, {"show", a, p, "--editor", "--editor-command", "nonexistent-editor"}} {
				code, out, stderr := run(t, "", args...)
				if code != 2 || out != "" || !strings.Contains(stderr, "NUL") {
					t.Fatalf("%q: %d %q %q", args, code, out, stderr)
				}
			}
			if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatalf("output was published: %v", err)
			}
		})
	}
}

func TestCLIOutput(t *testing.T) {
	d := t.TempDir()
	a := put(t, filepath.Join(d, "a"), "hello cat\r\n")
	b := put(t, filepath.Join(d, "b"), "hello dog\r\n")
	p, outPath := filepath.Join(d, "change.dcd"), filepath.Join(d, "result")
	other := put(t, filepath.Join(d, "other"), "hello rat\r\n")
	for _, tc := range []struct {
		args []string
		code int
		err  string
	}{
		{[]string{"create", a, b, "--output", p}, 0, ""},
		{[]string{"apply", a, p, "--output", outPath}, 0, ""},
		{[]string{"apply", a, p, "--output", a}, 2, "output"},
		{[]string{"apply", a, p, "--output", outPath}, 2, "output"},
		{[]string{"apply", other, p, "--output", "-"}, 2, "base"},
		{[]string{"diff", "-", "-"}, 2, "stdin"},
		{[]string{"diff", a, b, "--editor-command", "code"}, 2, "--editor"},
		{[]string{"diff", a, b, "--color", "invalid"}, 2, "color"},
		{[]string{"create", a, b, "--output", "-"}, 2, "output"},
	} {
		code, out, stderr := run(t, "", tc.args...)
		if code != tc.code || out != "" || (tc.err == "" && stderr != "") || !strings.Contains(stderr, tc.err) {
			t.Fatalf("%q: %d %q %q", tc.args, code, out, stderr)
		}
	}
	for _, tc := range []struct{ p, want string }{{a, "hello cat\r\n"}, {outPath, "hello dog\r\n"}} {
		got, err := os.ReadFile(tc.p)
		if err != nil || string(got) != tc.want {
			t.Fatalf("%s: %q %v", tc.p, got, err)
		}
	}
	if link := filepath.Join(d, "output-link"); os.Symlink(a, link) == nil {
		if code, _, _ := run(t, "", "apply", a, p, "--output", link); code != 2 {
			t.Fatal("overwrote symlink")
		}
	}
	c1, d1, e1 := run(t, "", "diff", a, b)
	c2, d2, e2 := run(t, "", "show", a, p)
	_, d1, _ = strings.Cut(d1, "@@")
	_, d2, _ = strings.Cut(d2, "@@")
	if c1 != 1 || c2 != 1 || d1 == "" || d1 != d2 || e1 != "" || e2 != "" {
		t.Fatalf("diff/show differ: %q %q", d1, d2)
	}
}

func TestCLIControlEscape(t *testing.T) {
	for _, tc := range []struct{ name, before, after, want string }{
		{"CR to text", "line\r\n", "line\\r\n", "-line\\r\n+line\\\\r\n"},
		{"text to CR", "line\\r\n", "line\r\n", "-line\\\\r\n+line\\r\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := put(t, filepath.Join(t.TempDir(), "after.txt"), tc.after)
			code, out, stderr := run(t, tc.before, "diff", "-", target, "--color=never")
			_, got, _ := strings.Cut(out, "@@ -1 +1 @@\n")
			if code != 1 || stderr != "" || got != tc.want {
				t.Fatalf("diff = %d, %q, %q; want body %q", code, out, stderr, tc.want)
			}
		})
	}
}

func TestCLIColorAndControls(t *testing.T) {
	a := put(t, filepath.Join(t.TempDir(), "a"), "old\x1b[2J\u202e\n")
	b := put(t, filepath.Join(t.TempDir(), "b"), "new\r\n")
	for _, tc := range []struct {
		mode    string
		colored bool
	}{{"auto", false}, {"never", false}, {"always", true}} {
		code, out, stderr := run(t, "", "diff", a, b, "--color", tc.mode)
		if code != 1 || stderr != "" || strings.ContainsRune(out, '\u202e') || strings.Contains(out, "\x1b[2J") || !strings.Contains(out, "\\x1b[2J\\u202e") || strings.Contains(out, "\x1b[31m") != tc.colored {
			t.Fatalf("%s: %d %q %q", tc.mode, code, out, stderr)
		}
	}
	c, out, stderr := run(t, "", "diff", filepath.Join(t.TempDir(), "bad\x1b[2J"), b)
	if c != 2 || out != "" || strings.ContainsRune(stderr, '\x1b') || !strings.Contains(stderr, "\\x1b[2J") {
		t.Fatalf("unsafe error output: %d %q %q", c, out, stderr)
	}
}
