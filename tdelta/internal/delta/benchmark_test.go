package delta_test

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/w00j00ng/goproj/tdelta/internal/delta"
)

func BenchmarkFiles(b *testing.B) {
	for _, size := range []int{1 << 10, 64 << 10, 1 << 20, 8 << 20} {
		b.Run(fmt.Sprintf("%dKiB", size>>10), func(b *testing.B) {
			base := text(size, 1, 64)
			edit := bytes.Clone(base)
			edit[size/2] = '!'
			repeatedA := bytes.Repeat([]byte("a\n"), size/2)
			repeatedB := bytes.Repeat([]byte("b\n"), size/2)
			oneLine := bytes.Repeat([]byte("a"), size)
			oneLineEdit := bytes.Clone(oneLine)
			oneLineEdit[size/2] = 'b'
			for _, tc := range []struct {
				name string
				a, b []byte
			}{
				{"same", base, base},
				{"small-edit", base, edit},
				{"large-deletion", base, base[size/4 : 3*size/4]},
				{"replacement", base, text(size, 2, 64)},
				{"repeated", repeatedA, repeatedB},
				{"one-line", oneLine, oneLineEdit},
			} {
				b.Run(tc.name, func(b *testing.B) {
					benchmarkPair(b, tc.a, tc.b)
				})
			}
		})
	}
}

func BenchmarkCompareBoundary(b *testing.B) {
	base, target := bytes.Repeat([]byte("a\n"), 500), bytes.Repeat([]byte("b\n"), 500)
	b.Run("at-limit", func(b *testing.B) { benchmarkPair(b, base, target) })
	b.Run("over-limit", func(b *testing.B) {
		benchmarkPair(b, base, append(bytes.Clone(target), 'b', '\n'))
	})
	// Keep the line count at the boundary, with shared long prefixes inside changed rows.
	a := append(bytes.Repeat([]byte("p"), 16000), 'a', '\n')
	c := append(bytes.Repeat([]byte("p"), 16000), 'b', '\n')
	b.Run("long-prefix-at-limit", func(b *testing.B) {
		benchmarkPair(b, bytes.Repeat(a, 500), bytes.Repeat(c, 500))
	})
}

func text(size int, seed uint64, width int) []byte {
	data := make([]byte, size)
	r := rand.New(rand.NewPCG(seed, 0))
	for i := range data {
		if i%width == width-1 {
			data[i] = '\n'
		} else {
			data[i] = byte(' ' + r.IntN(95))
		}
	}
	return data
}

func benchmarkPair(b *testing.B, base, target []byte) {
	b.Helper()
	patch, err := delta.Create(base, target)
	if err != nil {
		b.Fatal(err)
	}
	var compressed bytes.Buffer
	z := zlib.NewWriter(&compressed)
	if _, err := z.Write(target); err != nil {
		b.Fatal(err)
	}
	if err := z.Close(); err != nil {
		b.Fatal(err)
	}
	sizes := func(b *testing.B) {
		b.ReportMetric(float64(len(target)), "target-B")
		b.ReportMetric(float64(compressed.Len()), "zlib-target-B")
		b.ReportMetric(float64(len(patch)), "delta-B")
	}
	b.Run("Create", func(b *testing.B) {
		for b.Loop() {
			if _, err := delta.Create(base, target); err != nil {
				b.Fatal(err)
			}
		}
		sizes(b)
	})
	b.Run("Apply", func(b *testing.B) {
		for b.Loop() {
			if _, err := delta.Apply(base, patch); err != nil {
				b.Fatal(err)
			}
		}
		sizes(b)
	})
}
