// Package routingmatch shares ordered Xray matcher semantics between managed
// routing validation, destination previews and graph path projections.
package routingmatch

import (
	"fmt"
	"net/netip"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type Match struct {
	Domains   []string `json:"domain,omitempty"`
	IPs       []string `json:"ip,omitempty"`
	Port      string   `json:"port,omitempty"`
	Networks  []string `json:"network,omitempty"`
	Protocols []string `json:"protocol,omitempty"`
}
type Rule struct {
	ID         string `json:"id"`
	From       string `json:"from"`
	To         string `json:"to"`
	TemplateID string `json:"templateId"`
	Match      Match  `json:"match"`
}
type Destination struct {
	Domain   string `json:"domain,omitempty"`
	IP       string `json:"ip,omitempty"`
	Port     uint16 `json:"port"`
	Network  string `json:"network"`
	Protocol string `json:"protocol,omitempty"`
}
type Resolver func(string) ([]string, error)
type interval struct{ lo, hi uint16 }
type atom struct {
	cost        int
	kind, value string
	re          *regexp.Regexp
}
type compiled struct {
	id      string
	match   Match
	domains []atom
	ips     []netip.Prefix
	ports   []interval
}

func compile(m Match, resolve Resolver, budget *int) (compiled, error) {
	c := compiled{match: m}
	if len(m.Domains) > 4096 || len(m.IPs) > 4096 || len(m.Networks) > 2 || len(m.Protocols) > 16 {
		return c, fmt.Errorf("matcher exceeds bounded analysis limits")
	}
	for _, value := range m.Domains {
		values := []string{value}
		if strings.HasPrefix(value, "geosite:") {
			if resolve == nil {
				return c, fmt.Errorf("unresolved selector %s", value)
			}
			var err error
			values, err = resolve(value)
			if err != nil {
				return c, err
			}
			if len(values) == 0 || len(values) > 100000 {
				return c, fmt.Errorf("empty or oversized selector %s", value)
			}
		}
		for _, value := range values {
			a, err := parseAtom(value)
			if err != nil {
				return c, err
			}
			*budget -= a.cost + 1
			if *budget < 0 {
				return c, errAnalysisBudget
			}
			c.domains = append(c.domains, a)
		}
	}
	for _, value := range m.IPs {
		p, err := netip.ParsePrefix(value)
		if err != nil || p.Addr().Is4In6() {
			return c, fmt.Errorf("IP matcher requires a literal CIDR: %s", value)
		}
		c.ips = append(c.ips, p.Masked())
	}
	var err error
	c.ports, err = parsePorts(m.Port)
	if err != nil {
		return c, err
	}
	for _, n := range m.Networks {
		if n != "tcp" && n != "udp" {
			return c, fmt.Errorf("unsupported network %s", n)
		}
	}
	for _, p := range m.Protocols {
		if !regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`).MatchString(p) {
			return c, fmt.Errorf("invalid protocol matcher")
		}
	}
	return c, nil
}
func parsePorts(raw string) ([]interval, error) {
	if raw == "" {
		return []interval{{1, 65535}}, nil
	}
	if len(raw) > 512 {
		return nil, fmt.Errorf("port matcher too long")
	}
	out := []interval{}
	for _, part := range strings.Split(raw, ",") {
		a, b, has := strings.Cut(part, "-")
		lo, e := strconv.ParseUint(a, 10, 16)
		if !has {
			b = a
		}
		hi, f := strconv.ParseUint(b, 10, 16)
		if e != nil || f != nil || lo == 0 || hi < lo {
			return nil, fmt.Errorf("invalid port range %s", part)
		}
		out = append(out, interval{uint16(lo), uint16(hi)})
	}
	slices.SortFunc(out, func(a, b interval) int { return int(a.lo) - int(b.lo) })
	merged := []interval{}
	for _, r := range out {
		last := len(merged) - 1
		if last >= 0 && uint32(r.lo) <= uint32(merged[last].hi)+1 {
			merged[last].hi = max(merged[last].hi, r.hi)
		} else {
			merged = append(merged, r)
		}
	}
	return merged, nil
}
func (c compiled) matches(d Destination) bool {
	if len(c.domains) > 0 {
		found := false
		for _, a := range c.domains {
			found = found || a.matches(d.Domain)
		}
		if !found {
			return false
		}
	}
	if len(c.ips) > 0 {
		ip, e := netip.ParseAddr(d.IP)
		if e != nil {
			return false
		}
		ip = ip.Unmap()
		found := false
		for _, p := range c.ips {
			found = found || p.Contains(ip)
		}
		if !found {
			return false
		}
	}
	found := false
	for _, r := range c.ports {
		found = found || (d.Port >= r.lo && d.Port <= r.hi)
	}
	if !found {
		return false
	}
	return member(c.match.Networks, d.Network) && member(c.match.Protocols, d.Protocol)
}
func member(values []string, v string) bool {
	if len(values) == 0 {
		return true
	}
	for _, x := range values {
		if x == v {
			return true
		}
	}
	return false
}
