package platform

import (
	"fmt"
	"strconv"
	"strings"
)

func parseLinuxStarttime(data []byte, pid uint32) (uint64, error) {
	s := string(data)
	open, close := strings.IndexByte(s, '('), strings.LastIndexByte(s, ')')
	if open <= 0 || close <= open {
		return 0, fmt.Errorf("invalid proc stat comm")
	}
	n, err := strconv.ParseUint(strings.TrimSpace(s[:open]), 10, 32)
	if err != nil || uint32(n) != pid {
		return 0, fmt.Errorf("proc stat PID mismatch")
	}
	fields := strings.Fields(s[close+1:])
	if len(fields) < 20 {
		return 0, fmt.Errorf("truncated proc stat")
	}
	return strconv.ParseUint(fields[19], 10, 64)
}

func linuxSocketInode(link string) uint64 {
	if !strings.HasPrefix(link, "socket:[") || !strings.HasSuffix(link, "]") {
		return 0
	}
	n, err := strconv.ParseUint(link[8:len(link)-1], 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func linuxRestrictedProcfs(mountinfo, root string) bool {
	unescape := strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`)
	for mountinfo != "" {
		var line string
		line, mountinfo, _ = strings.Cut(mountinfo, "\n")
		if len(line) > 64<<10 {
			continue
		}
		left, right, ok := strings.Cut(line, " - ")
		fields, fs := strings.Fields(left), strings.Fields(right)
		if !ok || len(fields) < 6 || len(fs) < 3 || fs[0] != "proc" || unescape.Replace(fields[4]) != root {
			continue
		}
		for _, option := range strings.Split(fields[5]+","+fs[2], ",") {
			if value, ok := strings.CutPrefix(option, "hidepid="); ok && value != "0" && value != "off" {
				return true
			}
		}
	}
	return false
}
