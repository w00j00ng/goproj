package platform

import (
	"net/netip"
	"testing"

	"github.com/w00j00ng/goproj/portwho/internal/inspect"
)

func TestDarwinIdentityRequiresNativeIdentityAndEndpoint(t *testing.T) {
	listener := inspect.Socket{Family: "ipv6", Address: netip.MustParseAddr("fe80::1"), Port: 8080, ScopeID: 4, State: "LISTEN", SocketToken: 11, PCBToken: 22, Generation: 33}
	tests := []struct {
		name      string
		change    func(*inspect.Socket)
		wantMatch bool
	}{
		{"same socket", func(*inspect.Socket) {}, true},
		{"different socket at same endpoint", func(s *inspect.Socket) { s.SocketToken++ }, false},
		{"different pcb", func(s *inspect.Socket) { s.PCBToken++ }, false},
		{"reused token", func(s *inspect.Socket) { s.Generation++ }, false},
		{"missing socket token", func(s *inspect.Socket) { s.SocketToken = 0 }, false},
		{"missing pcb token", func(s *inspect.Socket) { s.PCBToken = 0 }, false},
		{"missing generation", func(s *inspect.Socket) { s.Generation = 0 }, false},
		{"different family", func(s *inspect.Socket) { s.Family = "ipv4" }, false},
		{"different address", func(s *inspect.Socket) { s.Address = netip.MustParseAddr("fe80::2") }, false},
		{"different port", func(s *inspect.Socket) { s.Port++ }, false},
		{"different scope", func(s *inspect.Socket) { s.ScopeID++ }, false},
		{"not listening", func(s *inspect.Socket) { s.State = "ESTABLISHED" }, false},
	}
	want, ok := darwinSocketIdentity(listener)
	if !ok {
		t.Fatal("listener has no identity")
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := listener
			tt.change(&candidate)
			got, valid := darwinSocketIdentity(candidate)
			if match := valid && got == want; match != tt.wantMatch {
				t.Fatalf("matched = %v, want %v", match, tt.wantMatch)
			}
		})
	}
}
