package inspect

import "net/netip"

type Runtime struct {
	OS           string `json:"os"`
	Arch         string `json:"arch"`
	Backend      string `json:"backend"`
	SocketScope  string `json:"socket_scope"`
	NetworkScope string `json:"network_scope"`
	PIDScope     string `json:"pid_scope"`
}

type Query struct {
	Ports       []uint16 `json:"ports"`
	Family      string   `json:"family"`
	IncludePath bool     `json:"include_path"`
}

// Socket keeps native identity until ownership has been resolved.
type Socket struct {
	ID          string
	Family      string
	Address     netip.Addr
	Port        uint16
	ScopeID     uint32
	State       string
	PIDs        []uint32
	Inode       uint64
	Cookie      [2]uint32
	UID         uint32
	SocketToken uint64
	PCBToken    uint64
	Generation  uint64
}

type Diagnostic struct {
	Stage   string  `json:"stage"`
	Family  *string `json:"family"`
	Code    string  `json:"code"`
	OSCode  *int64  `json:"os_code"`
	Message string  `json:"message"`
}

type FamilyResult struct {
	Sockets []Socket
	Error   *Diagnostic
}

type Provider interface {
	Runtime() Runtime
	Collect(families []string) map[string]FamilyResult
	Resolve(sockets []Socket) (map[string][]uint32, []string)
	ImagePath(pid uint32) (string, error)
}

type Process struct {
	Name   *string `json:"name"`
	Path   *string `json:"path"`
	Status string  `json:"status"`
	Reason *string `json:"reason"`
}

type Owner struct {
	PID     uint32  `json:"pid"`
	Process Process `json:"process"`
}

type Local struct {
	Address string `json:"address"`
	Port    uint16 `json:"port"`
	ScopeID uint32 `json:"scope_id"`
}

type Listener struct {
	Protocol        string  `json:"protocol"`
	Family          string  `json:"family"`
	State           string  `json:"state"`
	Local           Local   `json:"local"`
	OwnershipStatus string  `json:"ownership_status"`
	Owners          []Owner `json:"owners"`
}

type Collection struct {
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at"`
	Complete   bool   `json:"complete"`
	IPv4       string `json:"ipv4"`
	IPv6       string `json:"ipv6"`
}

type Ownership struct {
	Mode        string   `json:"mode"`
	Method      string   `json:"method"`
	Limitations []string `json:"limitations"`
}

type Result struct {
	SchemaVersion int          `json:"schema_version"`
	Runtime       Runtime      `json:"runtime"`
	Query         Query        `json:"query"`
	Collection    Collection   `json:"collection"`
	Ownership     Ownership    `json:"ownership"`
	Listeners     []Listener   `json:"listeners"`
	Errors        []Diagnostic `json:"errors"`
}
