package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
	"unicode"

	"github.com/w00j00ng/goproj/portwho/internal/inspect"
)

func Write(out, errout io.Writer, r inspect.Result, mode string, code int) error {
	for _, e := range r.Errors {
		if _, err := fmt.Fprintf(errout, "portwho: %s: %s\n", e.Code, e.Message); err != nil {
			return err
		}
	}
	if mode == "pids" {
		if code != 0 {
			return nil
		}
		pids := []uint32{}
		for _, l := range r.Listeners {
			for _, o := range l.Owners {
				pids = append(pids, o.PID)
			}
		}
		slices.Sort(pids)
		pids = slices.Compact(pids)
		var b strings.Builder
		for _, pid := range pids {
			fmt.Fprintln(&b, pid)
		}
		return write(out, []byte(b.String()))
	}
	if mode == "json" {
		data, err := json.Marshal(r)
		if err != nil {
			return err
		}
		return write(out, append(data, '\n'))
	}
	warned := map[uint32]bool{}
	for _, l := range r.Listeners {
		for _, o := range l.Owners {
			if o.Process.Status == "unavailable" && !warned[o.PID] {
				warned[o.PID] = true
				if _, err := fmt.Fprintf(errout, "portwho: image metadata unavailable for PID %d\n", o.PID); err != nil {
					return err
				}
			}
		}
	}
	if len(r.Ownership.Limitations) > 0 {
		if _, err := fmt.Fprintf(errout, "portwho: owner visibility is limited (%s); resolved sockets can still succeed\n", strings.Join(r.Ownership.Limitations, ", ")); err != nil {
			return err
		}
	}
	if len(r.Listeners) == 0 {
		if r.Collection.Complete {
			return write(out, []byte("No matching TCP listeners found.\n"))
		}
		return nil
	}
	var b bytes.Buffer
	w := tabwriter.NewWriter(&b, 0, 2, 2, ' ', 0)
	header := "PROTO\tFAMILY\tLOCAL\tPID\tPROCESS\tOWNER"
	if r.Query.IncludePath {
		header += "\tPATH"
	}
	fmt.Fprintln(w, header)
	for _, l := range r.Listeners {
		address := l.Local.Address
		if l.Family == "ipv6" {
			if l.Local.ScopeID != 0 {
				address += "%" + strconv.FormatUint(uint64(l.Local.ScopeID), 10)
			}
			address = "[" + address + "]"
		}
		local := address + ":" + strconv.Itoa(int(l.Local.Port))
		owners := l.Owners
		if len(owners) == 0 {
			owners = []inspect.Owner{{}}
		}
		for _, o := range owners {
			pid, name, imagePath := "?", "?", "?"
			if len(l.Owners) > 0 {
				pid = strconv.FormatUint(uint64(o.PID), 10)
				name = "<unavailable>"
				imagePath = name
				if o.Process.Reason != nil && *o.Process.Reason == "access_denied" {
					name = "<access-denied>"
					imagePath = name
				}
				if o.Process.Name != nil {
					name = escape(*o.Process.Name)
				}
				if o.Process.Path != nil {
					imagePath = escape(*o.Process.Path)
				}
			}
			fmt.Fprintf(w, "tcp\t%s\t%s\t%s\t%s\t%s", l.Family, local, pid, name, l.OwnershipStatus)
			if r.Query.IncludePath {
				fmt.Fprintf(w, "\t%s", imagePath)
			}
			fmt.Fprintln(w)
		}
	}
	if err := w.Flush(); err != nil {
		return err
	}
	return write(out, b.Bytes())
}

func write(w io.Writer, p []byte) error {
	n, err := w.Write(p)
	if err == nil && n != len(p) {
		return io.ErrShortWrite
	}
	return err
}

func escape(s string) string {
	var b strings.Builder
	for _, r := range strings.ToValidUTF8(s, "\uFFFD") {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == '\u2028' || r == '\u2029' {
			switch r {
			case '\n':
				b.WriteString(`\n`)
			case '\r':
				b.WriteString(`\r`)
			case '\t':
				b.WriteString(`\t`)
			default:
				fmt.Fprintf(&b, `\u%04x`, r)
			}
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
