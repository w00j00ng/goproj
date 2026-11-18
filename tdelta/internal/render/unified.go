package render

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/w00j00ng/goproj/tdelta/internal/compare"
)

const contextLines = 3

// Write emits a unified display of validated changes.
func Write(out io.Writer, a, b []byte, changes []compare.Change, from, to string, color bool) error {
	if len(changes) == 0 {
		return nil
	}
	w := bufio.NewWriter(out)
	if _, err := fmt.Fprintf(w, "--- %s\n+++ %s\n", Escape(from, false), Escape(to, false)); err != nil {
		return err
	}
	var aPos, bPos, aLine, bLine int
	for i := 0; i < len(changes); {
		j := i + 1
		for j < len(changes) && compare.LineCount(a[changes[j-1].End:changes[j].Start]) <= 2*contextLines {
			j++
		}
		first, last := changes[i], changes[j-1]
		start, end := first.Start, last.End
		for range contextLines {
			if start > 0 {
				start = compare.LastLine(a[:start])
			}
			end += compare.NextLine(a[end:])
		}
		newStart, newEnd := first.NewStart-(first.Start-start), last.NewEnd+end-last.End
		aLine += compare.LineCount(a[aPos:start])
		bLine += compare.LineCount(b[bPos:newStart])
		aCount, bCount := compare.LineCount(a[start:end]), compare.LineCount(b[newStart:newEnd])
		if _, err := fmt.Fprintf(w, "@@ -%s +%s @@\n", lineRange(aLine, aCount), lineRange(bLine, bCount)); err != nil {
			return err
		}
		cursor := start
		for _, c := range changes[i:j] {
			if err := lines(w, a[cursor:c.Start], ' ', false); err != nil {
				return err
			}
			if err := lines(w, a[c.Start:c.End], '-', color); err != nil {
				return err
			}
			if err := lines(w, b[c.NewStart:c.NewEnd], '+', color); err != nil {
				return err
			}
			cursor = c.End
		}
		if err := lines(w, a[cursor:end], ' ', false); err != nil {
			return err
		}
		aPos, bPos = end, newEnd
		aLine, bLine = aLine+aCount, bLine+bCount
		i = j
	}
	return w.Flush()
}

func lineRange(index, count int) string {
	if count == 1 {
		return strconv.Itoa(index + 1)
	}
	if count > 0 {
		index++
	}
	return fmt.Sprintf("%d,%d", index, count)
}

func lines(w *bufio.Writer, data []byte, prefix byte, color bool) error {
	style := ""
	if color {
		style = "\x1b[31m"
		if prefix == '+' {
			style = "\x1b[32m"
		}
	}
	for len(data) > 0 {
		n := compare.NextLine(data)
		line := data[:n]
		data = data[n:]
		hasNewline := line[len(line)-1] == '\n'
		if hasNewline {
			line = line[:len(line)-1]
		}
		if style != "" {
			if _, err := w.WriteString(style); err != nil {
				return err
			}
		}
		if err := w.WriteByte(prefix); err != nil {
			return err
		}
		if _, err := w.WriteString(Escape(string(line), true)); err != nil {
			return err
		}
		if style != "" {
			if _, err := w.WriteString("\x1b[0m"); err != nil {
				return err
			}
		}
		if err := w.WriteByte('\n'); err != nil {
			return err
		}
		if !hasNewline {
			if _, err := w.WriteString("\\ No newline at end of file\n"); err != nil {
				return err
			}
		}
	}
	return nil
}

func Escape(s string, diffLine bool) string {
	escape := func(r rune) bool {
		return (diffLine && r == '\\') ||
			(unicode.IsControl(r) && !(diffLine && r == '\t')) ||
			unicode.Is(unicode.Bidi_Control, r)
	}
	first := strings.IndexFunc(s, func(r rune) bool { return r == utf8.RuneError || escape(r) })
	if first < 0 {
		return s
	}
	var b strings.Builder
	b.WriteString(s[:first])
	for s = s[first:]; len(s) > 0; {
		r, size := utf8.DecodeRuneInString(s)
		switch {
		case r == utf8.RuneError && size == 1:
			fmt.Fprintf(&b, "\\x%02x", s[0])
		case escape(r):
			q := strconv.QuoteRuneToASCII(r)
			b.WriteString(q[1 : len(q)-1])
		default:
			b.WriteRune(r)
		}
		s = s[size:]
	}
	return b.String()
}
