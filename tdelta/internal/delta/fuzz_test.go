package delta_test

import (
	"bytes"
	"testing"
	"unicode/utf8"

	"github.com/w00j00ng/goproj/tdelta/internal/delta"
)

func FuzzApply(f *testing.F) {
	base, target := []byte("base"), []byte("BASE")
	valid := fixture(f, base, target, []byte{2, 4, 'B', 'A', 'S', 'E'})
	f.Add(base, valid)
	f.Add(base, valid[:len(valid)-1])
	f.Fuzz(func(t *testing.T, base, payload []byte) {
		got, err := delta.Apply(base, payload)
		if err != nil {
			if got != nil {
				t.Fatal("failure returned a result")
			}
			return
		}
		if !utf8.Valid(got) || bytes.IndexByte(got, 0) >= 0 {
			t.Fatal("success returned invalid text")
		}
	})
}
