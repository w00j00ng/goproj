package cmd_test

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestCLIEditor(t *testing.T) {
	d := t.TempDir()
	temp := filepath.Join(d, "한글 space & %TDELTA_TEST% !")
	if err := os.Mkdir(temp, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", temp)
	t.Setenv("TEMP", temp)
	t.Setenv("TMP", temp)
	t.Setenv("TDELTA_TEST", "expanded")
	t.Setenv("TDELTA_EDITOR", "")
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TDELTA_TEST_HELPER", self)
	t.Setenv("TDELTA_TEST_TEMP", temp)
	t.Setenv("PATH", d+string(os.PathListSeparator)+os.Getenv("PATH"))
	editor := filepath.Join(d, "test-editor")
	if runtime.GOOS == "windows" {
		editor += ".cmd"
		put(t, editor, "@echo off\r\nsetlocal DisableDelayedExpansion\r\n\"%TDELTA_TEST_HELPER%\" -test.run=^TestEditorHelper$ -- %*\r\nexit /b %errorlevel%\r\n")
	} else {
		put(t, editor, "#!/bin/sh\nexec \"$TDELTA_TEST_HELPER\" -test.run=^TestEditorHelper$ -- \"$@\"\n")
		if err := os.Chmod(editor, 0700); err != nil {
			t.Fatal(err)
		}
	}
	a := put(t, filepath.Join(d, "a"), "old\n")
	b := put(t, filepath.Join(d, "b"), "new\n")
	p := filepath.Join(d, "change.dcd")
	if code, _, stderr := run(t, "", "create", a, b, "--output", p); code != 0 {
		t.Fatalf("create: %s", stderr)
	}
	pBefore, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, subcommand, explicit  string
		env, visual, fallback, fail bool
		wantCode                    int
	}{
		{"explicit wins", "diff", "test-editor --wait", true, true, true, false, 1},
		{"program env wins", "show", "", true, true, true, false, 1},
		{"visual wins", "diff", "", false, true, true, false, 1},
		{"editor fallback", "diff", "", false, false, true, false, 1},
		{"without config", "diff", "", false, false, false, false, 2},
		{"nonzero editor", "diff", "test-editor --wait", false, false, false, true, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// A wrong selection attempts to launch an executable that does not exist.
			config := func(on bool, valid bool) string {
				if !on {
					return ""
				}
				if valid {
					return "test-editor --wait"
				}
				return "nonexistent-editor"
			}
			t.Setenv("TDELTA_EDITOR", config(tc.env, tc.explicit == ""))
			t.Setenv("VISUAL", config(tc.visual, tc.explicit == "" && !tc.env))
			t.Setenv("EDITOR", config(tc.fallback, tc.explicit == "" && !tc.env && !tc.visual))
			marker := filepath.Join(d, tc.name+".opened")
			t.Setenv("TDELTA_TEST_MARKER", marker)
			t.Setenv("TDELTA_TEST_FAIL", "")
			if tc.fail {
				t.Setenv("TDELTA_TEST_FAIL", "1")
			}
			second := b
			if tc.subcommand == "show" {
				second = p
			}
			args := []string{tc.subcommand, a, second, "--editor", "--color=always"}
			if tc.explicit != "" {
				args = append(args, "--editor-command", tc.explicit)
			}
			code, out, stderr := run(t, "", args...)
			if code != tc.wantCode {
				t.Fatalf("diff = %d %q %q; want %d", code, out, stderr, tc.wantCode)
			}
			wantsLaunch := tc.env || tc.visual || tc.fallback || tc.explicit != ""
			f, e := os.ReadFile(marker)
			if wantsLaunch {
				if e != nil {
					t.Fatalf("editor did not complete its file check: %v %q %q", e, out, stderr)
				}
				if _, err := os.Stat(string(f)); !os.IsNotExist(err) {
					t.Fatalf("temp file still exists: %v", err)
				}
			} else if !os.IsNotExist(e) {
				t.Fatalf("unexpected editor launch: %q %v", f, e)
			}
			if entries, e := os.ReadDir(temp); e != nil || len(entries) != 0 {
				t.Fatalf("temp dir not empty: %v %v", entries, e)
			}
			for path, want := range map[string]string{a: "old\n", b: "new\n", p: string(pBefore)} {
				got, e := os.ReadFile(path)
				if e != nil || string(got) != want {
					t.Fatalf("editor changed %q: %v", path, e)
				}
			}
		})
	}
}

func TestEditorHelper(t *testing.T) {
	i := slices.Index(os.Args, "--")
	if i < 0 {
		return
	}
	args := os.Args[i+1:]
	if len(args) != 2 || args[0] != "--wait" || (runtime.GOOS == "windows" && args[1] != "change.diff") {
		t.Fatalf("editor argv = %q", args)
	}
	path, err := filepath.Abs(args[1])
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(os.Getenv("TDELTA_TEST_TEMP"), path)
	if err != nil || !filepath.IsLocal(relative) {
		t.Fatalf("editor path = %q: %v", path, err)
	}
	diff, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(diff), "-old\n+new\n") || strings.ContainsRune(string(diff), '\x1b') {
		t.Fatalf("invalid editor diff: %q %v", diff, err)
	}
	put(t, path, "edited scratch\n")
	put(t, os.Getenv("TDELTA_TEST_MARKER"), path)
	if os.Getenv("TDELTA_TEST_FAIL") != "" {
		os.Exit(7)
	}
}
