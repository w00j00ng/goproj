package platform

import (
	"net/netip"

	"github.com/w00j00ng/goproj/portwho/internal/inspect"
)

type darwinIdentity struct {
	socket, pcb, generation uint64
	family                  string
	address                 netip.Addr
	port                    uint16
	scope                   uint32
}

func darwinSocketIdentity(s inspect.Socket) (darwinIdentity, bool) {
	if s.SocketToken == 0 || s.PCBToken == 0 || s.Generation == 0 || s.State != "LISTEN" {
		return darwinIdentity{}, false
	}
	return darwinIdentity{s.SocketToken, s.PCBToken, s.Generation, s.Family, s.Address, s.Port, s.ScopeID}, true
}
