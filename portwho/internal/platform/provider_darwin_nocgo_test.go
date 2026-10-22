//go:build darwin && !cgo && (amd64 || arm64)

package platform

import "testing"

func TestDarwinWithoutCGOReportsUnavailableForEveryRequestedFamily(t *testing.T) {
	p := New()
	got := p.Collect([]string{"ipv4", "ipv6"})
	for _, family := range []string{"ipv4", "ipv6"} {
		result := got[family]
		if result.Error == nil || result.Error.Code != "backend_unavailable" || len(result.Sockets) != 0 {
			t.Fatalf("%s = %+v", family, result)
		}
	}
	if p.Runtime().Backend != "unsupported" {
		t.Fatalf("backend = %q", p.Runtime().Backend)
	}
}
