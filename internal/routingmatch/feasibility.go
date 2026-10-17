package routingmatch

import (
	"net/netip"
	"slices"

	"go4.org/netipx"
)

type feasibility uint8

const (
	unknown feasibility = iota
	impossible
	possible
)

// feasible uses exact finite intersections for CIDRs/ports and conservative
// domain inclusion proofs. A concrete witness proves a loop. Analysis limits or
// unresolved regular-language intersections produce unknown and fail closed.
func feasible(positive, negative []compiled, budget *int) feasibility {
	for _, c := range append(slices.Clone(positive), negative...) {
		*budget -= len(c.domains) + len(c.ips) + len(c.ports) + 1
	}
	if *budget < 0 {
		return unknown
	}
	anchors, exact := domainAnchors(positive, budget)
	if anchors == nil {
		return unknown
	}
	if len(anchors) == 0 {
		return impossible
	}
	var builder netipx.IPSetBuilder
	builder.AddPrefix(netip.MustParsePrefix("0.0.0.0/0"))
	builder.AddPrefix(netip.MustParsePrefix("::/0"))
	ports := []interval{{1, 65535}}
	networks := []string{"tcp", "udp"}
	protocols := []string{""}
	for _, c := range positive {
		if len(c.ips) > 0 {
			var b netipx.IPSetBuilder
			for _, p := range c.ips {
				b.AddPrefix(p)
			}
			set, _ := b.IPSet()
			builder.Intersect(set)
		}
		ports = intersectPorts(ports, c.ports)
		if len(c.match.Networks) > 0 {
			networks = slices.DeleteFunc(networks, func(n string) bool { return !member(c.match.Networks, n) })
		}
		if len(c.match.Protocols) > 0 {
			if len(protocols) == 1 && protocols[0] == "" {
				protocols = slices.Clone(c.match.Protocols)
			} else {
				protocols = slices.DeleteFunc(protocols, func(p string) bool { return !member(c.match.Protocols, p) })
			}
		}
	}
	ips, err := builder.IPSet()
	if err != nil {
		return unknown
	}
	if len(ips.Prefixes()) == 0 || len(ports) == 0 || len(networks) == 0 || len(protocols) == 0 {
		return impossible
	}
	for _, n := range negative {
		if domainCovered(n, anchors, budget) && ipCovered(n, ips) && portsCovered(n.ports, ports) && stringsCovered(n.match.Networks, networks) && stringsCovered(n.match.Protocols, protocols) {
			return impossible
		}
	}
	domains := []string{"", "routing-check.invalid"}
	for _, a := range anchors {
		domains = append(domains, a.value)
		if a.kind == "domain" {
			domains = append(domains, "routing-check."+a.value)
		}
	}
	for _, c := range append(slices.Clone(positive), negative...) {
		for _, a := range c.domains {
			if a.kind != "regexp" {
				domains = append(domains, a.value, "routing-check."+a.value)
			}
		}
	}
	for _, d := range domains {
		for _, ip := range ips.Prefixes() {
			for _, port := range ports {
				for _, network := range networks {
					for _, protocol := range protocols {
						*budget--
						if *budget < 0 {
							return unknown
						}
						destination := Destination{Domain: d, IP: ip.Addr().String(), Port: port.lo, Network: network, Protocol: protocol}
						matched := true
						for _, c := range positive {
							matched = matched && c.matches(destination)
						}
						for _, c := range negative {
							matched = matched && !c.matches(destination)
						}
						if matched {
							return possible
						}
					}
				}
			}
		}
	}
	_ = exact // A nonempty symbolic set without a concrete witness stays unknown.
	return unknown
}
func stringsCovered(outer, inner []string) bool {
	if len(outer) == 0 {
		return true
	}
	for _, v := range inner {
		if v == "" || !slices.Contains(outer, v) {
			return false
		}
	}
	return true
}
func ipCovered(c compiled, inner *netipx.IPSet) bool {
	if len(c.ips) == 0 {
		return true
	}
	var b netipx.IPSetBuilder
	for _, p := range c.ips {
		b.AddPrefix(p)
	}
	set, _ := b.IPSet()
	var remaining netipx.IPSetBuilder
	remaining.AddSet(inner)
	remaining.RemoveSet(set)
	rest, _ := remaining.IPSet()
	return len(rest.Prefixes()) == 0
}
func intersectPorts(a, b []interval) []interval {
	out := []interval{}
	for _, x := range a {
		for _, y := range b {
			lo, hi := max(x.lo, y.lo), min(x.hi, y.hi)
			if lo <= hi {
				out = append(out, interval{lo, hi})
			}
		}
	}
	return out
}
func portsCovered(outer, inner []interval) bool {
	for _, r := range inner {
		cursor := uint32(r.lo)
		for cursor <= uint32(r.hi) {
			next := cursor
			for _, o := range outer {
				if uint32(o.lo) <= cursor && uint32(o.hi) >= cursor {
					next = max(next, uint32(o.hi)+1)
				}
			}
			if next == cursor {
				return false
			}
			cursor = next
		}
	}
	return true
}
