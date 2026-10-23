package platform

import (
	"fmt"
	"strings"
	"testing"
)

func linuxStat(pid uint32, start uint64) string {
	return fmt.Sprintf("%d (worker (with spaces)) S %s %d 0 0\n", pid, strings.Repeat("0 ", 18), start)
}

func TestLinuxProcStarttime(t *testing.T) {
	for _, tc := range []struct {
		name, data string
		pid        uint32
		want       uint64
		invalid    bool
	}{
		{"comm parentheses and spaces", linuxStat(123, 9988), 123, 9988, false},
		{"different pid", linuxStat(124, 9988), 123, 0, true},
		{"short stat", "123 (worker) S 1", 123, 0, true},
		{"bad starttime", strings.Replace(linuxStat(123, 9988), "9988", "-1", 1), 123, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseLinuxStarttime([]byte(tc.data), tc.pid)
			if (err != nil) != tc.invalid || got != tc.want {
				t.Fatalf("start=%d error=%v", got, err)
			}
		})
	}
}

func TestLinuxSocketLink(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  uint64
	}{
		{"socket:[123456]", 123456}, {"socket:[0]", 0}, {"socket:[-1]", 0},
		{"socket:[18446744073709551616]", 0}, {"socket:[12]junk", 0}, {"pipe:[12]", 0}, {"/tmp/socket:[12]", 0},
	} {
		if got := linuxSocketInode(tc.input); got != tc.want {
			t.Fatalf("%q got=%d want=%d", tc.input, got, tc.want)
		}
	}
}

func TestLinuxProcMountRestrictions(t *testing.T) {
	for _, tc := range []struct {
		options string
		want    bool
	}{
		{"rw", false}, {"rw,hidepid=0", false}, {"rw,hidepid=off", false}, {"rw,hidepid=2", true}, {"rw,hidepid=invisible", true},
	} {
		input := "1 2 0:1 / /proc rw,nosuid - proc proc " + tc.options + "\n"
		if got := linuxRestrictedProcfs(input, "/proc"); got != tc.want {
			t.Fatalf("%q restricted=%v", tc.options, got)
		}
	}
}
