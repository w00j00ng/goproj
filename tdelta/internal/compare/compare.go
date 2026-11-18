package compare

import (
	"bytes"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/hexops/gotextdiff/myers"
)

const (
	MaxText  = 8 << 20
	maxLines = 1000
)

var ErrLimit = errors.New("line comparison limit exceeded")

// Change replaces A[Start:End] with B[NewStart:NewEnd] at whole-line boundaries.
type Change struct {
	Start, End, NewStart, NewEnd int
}

func Validate(text []byte) error {
	switch {
	case len(text) > MaxText:
		return fmt.Errorf("text exceeds %d bytes", MaxText)
	case bytes.IndexByte(text, 0) >= 0:
		return errors.New("text contains NUL")
	case !utf8.Valid(text):
		return errors.New("text is not valid UTF-8")
	}
	return nil
}

func LineCount(text []byte) int {
	n := bytes.Count(text, []byte{'\n'})
	if len(text) > 0 && text[len(text)-1] != '\n' {
		n++
	}
	return n
}

// NextLine returns the number of bytes through the first line's terminator or EOF.
func NextLine(text []byte) int {
	if i := bytes.IndexByte(text, '\n'); i >= 0 {
		return i + 1
	}
	return len(text)
}

// LastLine returns the byte index of the last line, including an unterminated one.
func LastLine(text []byte) int {
	end := len(text)
	if end > 0 && text[end-1] == '\n' {
		end--
	}
	return bytes.LastIndexByte(text[:end], '\n') + 1
}

func Find(base, target []byte) ([]Change, error) {
	if err := Validate(base); err != nil {
		return nil, fmt.Errorf("base: %w", err)
	}
	if err := Validate(target); err != nil {
		return nil, fmt.Errorf("target: %w", err)
	}
	if bytes.Equal(base, target) {
		return nil, nil
	}
	start := 0
	for start < len(base) && start < len(target) {
		m, n := NextLine(base[start:]), NextLine(target[start:])
		if m != n || !bytes.Equal(base[start:start+m], target[start:start+n]) {
			break
		}
		start += m
	}
	aEnd, bEnd := len(base), len(target)
	for aEnd > start && bEnd > start {
		i, j := start+LastLine(base[start:aEnd]), start+LastLine(target[start:bEnd])
		if !bytes.Equal(base[i:aEnd], target[j:bEnd]) {
			break
		}
		aEnd, bEnd = i, j
	}
	a, b := base[start:aEnd], target[start:bEnd]
	if len(a) == 0 || len(b) == 0 {
		return []Change{{start, aEnd, start, bEnd}}, nil
	}
	if count := LineCount(a) + LineCount(b); count > maxLines {
		return nil, fmt.Errorf("%w: %d remaining lines (maximum %d)", ErrLimit, count, maxLines)
	}

	offsets := []int{0}
	for i := 0; i < len(a); {
		i += NextLine(a[i:])
		offsets = append(offsets, i)
	}
	var changes []Change
	var ai, bi int
	for _, edit := range myers.ComputeEdits("", string(a), string(b)) {
		from, to := edit.Span.Start(), edit.Span.End()
		if from.Column() != 1 || to.Column() != 1 || from.Line() < 1 || from.Line() > to.Line() || to.Line() > len(offsets) {
			return nil, errors.New("invalid line range in computed diff")
		}
		x, y := offsets[from.Line()-1], offsets[to.Line()-1]
		z := bi + x - ai
		end := z + len(edit.NewText)
		if x < ai || end > len(b) || !bytes.Equal(a[ai:x], b[bi:z]) || edit.NewText != string(b[z:end]) {
			return nil, errors.New("computed diff does not match target")
		}
		if n := len(changes); n > 0 && x == ai {
			changes[n-1].End = start + y
			changes[n-1].NewEnd = start + end
		} else {
			changes = append(changes, Change{start + x, start + y, start + z, start + end})
		}
		ai, bi = y, end
	}
	if !bytes.Equal(a[ai:], b[bi:]) {
		return nil, errors.New("computed diff does not match target suffix")
	}
	return changes, nil
}
