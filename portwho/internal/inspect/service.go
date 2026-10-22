package inspect

import (
	"errors"
	"net/netip"
	"os"
	"path"
	"slices"
	"strings"
	"time"
)

func Run(provider Provider, query Query, pidsOnly bool) (Result, int) {
	r := Result{SchemaVersion: 2, Runtime: provider.Runtime(), Query: query,
		Collection: Collection{Complete: true, IPv4: "not_requested", IPv6: "not_requested"},
		Ownership:  Ownership{Mode: "best_effort", Method: "fd_holder", Limitations: []string{}},
		Listeners:  []Listener{}, Errors: []Diagnostic{}}
	if r.Runtime.OS == "windows" {
		r.Ownership.Method = "reported_owner"
	}
	if r.Runtime.Backend == "unsupported" {
		r.Ownership.Method = "unavailable"
	}
	families := []string{"ipv4", "ipv6"}
	if query.Family != "both" {
		families = []string{query.Family}
	}
	r.Collection.StartedAt = time.Now().UTC().Format(time.RFC3339Nano)
	collected := provider.Collect(families)
	r.Collection.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
	selected := []Socket{}
	seenErrors := map[*Diagnostic]bool{}
	for _, family := range families {
		fr, ok := collected[family]
		if !ok {
			fr.Error = &Diagnostic{Stage: "collection", Family: &family, Code: "collection_failed", Message: "collector did not return the requested family"}
		}
		status := "ok"
		if fr.Error != nil {
			status = "failed"
			r.Collection.Complete = false
			if !seenErrors[fr.Error] {
				r.Errors = append(r.Errors, *fr.Error)
				seenErrors[fr.Error] = true
			}
		} else {
			for _, s := range fr.Sockets {
				if s.State == "LISTEN" && s.Family == family && (query.Ports == nil || slices.Contains(query.Ports, s.Port)) {
					selected = append(selected, s)
				}
			}
		}
		if family == "ipv4" {
			r.Collection.IPv4 = status
		} else {
			r.Collection.IPv6 = status
		}
	}
	owners := map[string][]uint32{}
	if len(selected) > 0 {
		var limitations []string
		owners, limitations = provider.Resolve(selected)
		slices.Sort(limitations)
		r.Ownership.Limitations = append(r.Ownership.Limitations, slices.Compact(limitations)...)
	}
	type endpoint struct {
		family  string
		address netip.Addr
		port    uint16
		scope   uint32
	}
	type group struct {
		total, mapped int
		pids          map[uint32]bool
	}
	groups := map[endpoint]*group{}
	for _, s := range selected {
		key := endpoint{s.Family, s.Address, s.Port, s.ScopeID}
		g := groups[key]
		if g == nil {
			g = &group{pids: map[uint32]bool{}}
			groups[key] = g
		}
		g.total++
		if len(owners[s.ID]) > 0 {
			g.mapped++
		}
		for _, pid := range owners[s.ID] {
			g.pids[pid] = true
		}
	}
	metadata := map[uint32]Process{}
	missing := false
	for key, g := range groups {
		status := "resolved"
		if g.mapped == 0 {
			status = "unresolved"
		} else if g.mapped < g.total {
			status = "partial"
		}
		if status != "resolved" {
			missing = true
		}
		l := Listener{Protocol: "tcp", Family: key.family, State: "LISTEN", Local: Local{Address: key.address.String(), Port: key.port, ScopeID: key.scope}, OwnershipStatus: status, Owners: []Owner{}}
		pids := make([]uint32, 0, len(g.pids))
		for pid := range g.pids {
			pids = append(pids, pid)
		}
		slices.Sort(pids)
		for _, pid := range pids {
			if !pidsOnly {
				if _, ok := metadata[pid]; !ok {
					metadata[pid] = image(provider, pid, query.IncludePath, r.Runtime.OS)
				}
			}
			l.Owners = append(l.Owners, Owner{PID: pid, Process: metadata[pid]})
		}
		r.Listeners = append(r.Listeners, l)
	}
	slices.SortFunc(r.Listeners, func(a, b Listener) int {
		if a.Local.Port != b.Local.Port {
			return int(a.Local.Port) - int(b.Local.Port)
		}
		if a.Family != b.Family {
			return strings.Compare(a.Family, b.Family)
		}
		if cmp := netip.MustParseAddr(a.Local.Address).Compare(netip.MustParseAddr(b.Local.Address)); cmp != 0 {
			return cmp
		}
		if a.Local.ScopeID < b.Local.ScopeID {
			return -1
		}
		if a.Local.ScopeID > b.Local.ScopeID {
			return 1
		}
		return 0
	})
	if missing {
		r.Errors = append(r.Errors, Diagnostic{Stage: "ownership", Code: "ownership_unresolved", Message: "some observed sockets could not be linked to a PID"})
	}
	if !r.Collection.Complete {
		return r, 3
	}
	if len(r.Listeners) == 0 {
		return r, 1
	}
	if missing {
		return r, 3
	}
	return r, 0
}

func image(provider Provider, pid uint32, includePath bool, goos string) Process {
	imagePath, err := provider.ImagePath(pid)
	if err != nil || imagePath == "" {
		reason := "os_error"
		if errors.Is(err, os.ErrPermission) {
			reason = "access_denied"
		}
		return Process{Status: "unavailable", Reason: &reason}
	}
	imagePath = strings.ToValidUTF8(imagePath, "\uFFFD")
	namePath := imagePath
	if goos == "windows" {
		namePath = strings.ReplaceAll(namePath, `\`, "/")
	}
	name := path.Base(namePath)
	p := Process{Name: &name, Status: "ok"}
	if includePath {
		p.Path = &imagePath
	}
	return p
}
