package delta_test

import (
	"bytes"
	"compress/zlib"
	"crypto/sha256"
	"encoding/binary"
	"io"
	"strings"
	"testing"

	"github.com/w00j00ng/goproj/tdelta/internal/delta"
)

func TestCreateApply(t *testing.T) {
	for _, tc := range []struct{ name, a, b string }{
		{"empty", "", ""},
		{"same", "same", "same"},
		{"insert", "", "가나다🙂"},
		{"delete", "remove all", ""},
		{"replace", "hello cat", "hello dog"},
		{"newlines", "a\r\nb\r\n", "a\nb\n"},
		{"no final newline", "a\n", "a"},
		{"BOM", "\ufeff가\n", "가\n"},
		{"unicode byte boundary", "a가\n", "a각\n"},
		{"repeated and separated", "a\nb\na\nc\na\nz", "first\na\nb\na\nD\nE\na\nz\nlast"},
		{"long identical edges", strings.Repeat("a\n", 800) + "x\n" + strings.Repeat("b\n", 800), strings.Repeat("a\n", 800) + "y\n" + strings.Repeat("b\n", 800)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := delta.Create([]byte(tc.a), []byte(tc.b))
			if err != nil {
				t.Fatal(err)
			}
			got, err := delta.Apply([]byte(tc.a), encoded)
			if err != nil || string(got) != tc.b {
				t.Fatalf("Apply = %q, %v; want %q", got, err, tc.b)
			}
		})
	}
}

func TestCreateCommands(t *testing.T) {
	long := strings.Repeat("L", 1<<19) + "x" + strings.Repeat("R", (1<<19)-1)
	overA, overB := strings.Repeat("a\n", 500), strings.Repeat("b\n", 501)
	for _, tc := range []struct {
		name, a, b string
		want       []byte
	}{
		{"one line", "abcdefgh\n", "abcdEfgh\n", []byte{1, 0, 4, 2, 1, 'E', 1, 5, 4}},
		{"one MiB", long, long[:1<<19] + "y" + long[(1<<19)+1:], []byte{1, 0, 0x80, 0x80, 0x20, 2, 1, 'y', 1, 0x81, 0x80, 0x20, 0xff, 0xff, 0x1f}},
		{"above compare limit", overA, overB, append([]byte{2, 0xea, 0x07}, overB...)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, b := []byte(tc.a), []byte(tc.b)
			encoded, err := delta.Create(a, b)
			if err != nil {
				t.Fatal(err)
			}
			got, err := delta.Apply(a, encoded)
			if err != nil || !bytes.Equal(got, b) {
				t.Fatalf("round trip failed: %v", err)
			}
			h := bytes.NewReader(encoded[5:])
			for range 2 {
				if _, err := binary.ReadUvarint(h); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := h.Seek(64, io.SeekCurrent); err != nil {
				t.Fatal(err)
			}
			z, err := zlib.NewReader(h)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := io.ReadAll(z)
			z.Close()
			if err != nil || !bytes.Equal(raw, tc.want) {
				t.Fatalf("commands mismatch: length=%d want=%d error=%v", len(raw), len(tc.want), err)
			}
		})
	}
}

func TestApplyFixture(t *testing.T) {
	base, target := []byte("abcd"), []byte("aXd")
	valid := fixture(t, base, target, []byte{1, 0, 1, 2, 1, 'X', 1, 3, 1})
	damage := bytes.Clone(valid)
	damage[len(damage)-1] ^= 1
	version := bytes.Clone(valid)
	version[4] = 2
	hash := bytes.Clone(valid)
	hash[40] ^= 1
	for _, tc := range []struct {
		name                string
		base, payload, want []byte
	}{
		{"independent fixture", base, valid, target},
		{"wrong base", []byte("abcE"), valid, nil},
		{"NUL base", []byte{0}, fixture(t, []byte{0}, target, append([]byte{2, 3}, target...)), nil},
		{"NUL result", base, fixture(t, base, []byte{0}, []byte{2, 1, 0}), nil},
		{"invalid UTF8 result", base, fixture(t, base, []byte{0xff}, []byte{2, 1, 0xff}), nil},
		{"bad checksum", base, damage, nil},
		{"bad target hash", base, hash, nil},
		{"version", base, version, nil},
		{"trailing data", base, append(bytes.Clone(valid), 1), nil},
		{"truncated header", base, valid[:6], nil},
		{"truncated checksum", base, valid[:len(valid)-2], nil},
		{"unknown command", base, fixture(t, base, target, []byte{3}), nil},
		{"truncated varint", base, fixture(t, base, target, []byte{1, 0x80}), nil},
		{"overflowing varint", base, fixture(t, base, target, append([]byte{1}, bytes.Repeat([]byte{0xff}, 11)...)), nil},
		{"truncated literal", base, fixture(t, base, target, []byte{2, 3, 'a'}), nil},
		{"copy outside base", base, fixture(t, base, target, []byte{1, 3, 3}), nil},
		{"zero length", base, fixture(t, base, target, []byte{2, 0}), nil},
		{"target length exceeded", base, fixture(t, base, target, []byte{1, 0, 4}), nil},
		{"oversize target header", base, append(binary.AppendUvarint([]byte("TDLT\x01\x04"), 8<<20|1), make([]byte, 64)...), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := delta.Apply(tc.base, tc.payload)
			if tc.want == nil {
				if err == nil || got != nil {
					t.Fatalf("invalid delta returned %q, %v", got, err)
				}
			} else if err != nil || !bytes.Equal(got, tc.want) {
				t.Fatalf("Apply = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func fixture(t testing.TB, base, target, commands []byte) []byte {
	t.Helper()
	h := binary.AppendUvarint([]byte("TDLT\x01"), uint64(len(base)))
	h = binary.AppendUvarint(h, uint64(len(target)))
	a, b := sha256.Sum256(base), sha256.Sum256(target)
	var buf bytes.Buffer
	buf.Write(h)
	buf.Write(a[:])
	buf.Write(b[:])
	z := zlib.NewWriter(&buf)
	if _, err := z.Write(commands); err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
