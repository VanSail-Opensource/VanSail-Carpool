package iplookup

import (
	"context"
	"encoding/json"
	"net/netip"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

type routeObservation struct {
	prefix string
	path   BGPPath
}

func asPath(raw string) []uint32 {
	values := strings.Fields(raw)
	if len(values) > 128 {
		return nil
	}
	path := make([]uint32, 0, len(values))
	for _, value := range values {
		n, err := strconv.ParseUint(value, 10, 32)
		if err != nil || n == 0 {
			return nil
		} // Do not invent adjacency across AS sets.
		path = append(path, uint32(n))
	}
	return path
}

func ripeRows(o object, state bool) []routeObservation {
	rows := []routeObservation{}
	d := nested(o, "data")
	if state {
		for _, r := range liveList(d, "bgp_state") {
			var path []uint32
			if json.Unmarshal(r["path"], &path) != nil {
				continue
			}
			rows = append(rows, routeObservation{textField(r, "target_prefix"), BGPPath{ASPath: path, Collector: textField(r, "source_id"), ObservedAt: textField(d, "timestamp")}})
		}
	} else {
		for _, collector := range liveList(d, "rrcs") {
			for _, r := range liveList(collector, "peers") {
				rows = append(rows, routeObservation{textField(r, "prefix"), BGPPath{ASPath: asPath(textField(r, "as_path")), Collector: textField(collector, "rrc"), Exchange: textField(collector, "scope"), ObservedAt: textField(r, "latest_time")}})
			}
		}
	}
	return rows
}

func routeviewsRows(data []byte) []routeObservation {
	var prefixes []object
	_ = json.Unmarshal(data, &prefixes)
	rows := []routeObservation{}
	for _, prefix := range prefixes {
		for _, r := range liveList(prefix, "reporting_peers") {
			rows = append(rows, routeObservation{textField(prefix, "prefix"), BGPPath{ASPath: asPath(textField(r, "as_path")), Collector: textField(r, "collector"), ObservedAt: textField(r, "timestamp")}})
		}
	}
	return rows
}

func buildTopology(ip string, rows []routeObservation, meta LiveMeta) BGPTopology {
	t := BGPTopology{LiveMeta: meta, ProjectionVersion: 2, Origins: []uint32{}, Nodes: []BGPNode{}, Edges: []BGPEdge{}, Paths: []BGPPath{}, RelationshipMeta: LiveMeta{Status: "empty", Source: "caida"}}
	address, _ := netip.ParseAddr(ip)
	longest := -1
	for _, r := range rows {
		p, err := netip.ParsePrefix(r.prefix)
		if err == nil && p.Bits() > 0 && p.Contains(address) && p.Bits() > longest {
			longest = p.Bits()
			t.Prefix = p.Masked().String()
		}
	}
	nodes, origins := map[uint32]bool{}, map[uint32]bool{}
	edges := map[[2]uint32]bool{}
	for _, r := range rows {
		p, err := netip.ParsePrefix(r.prefix)
		if err != nil || p.Masked().String() != t.Prefix {
			continue
		}
		if len(r.path.ASPath) == 0 {
			t.Truncated = true
			continue
		}
		valid := len(r.path.ASPath) <= 128
		newNodes := map[uint32]bool{}
		newEdges := map[[2]uint32]bool{}
		for _, asn := range r.path.ASPath {
			valid = valid && asn > 0
			if !nodes[asn] {
				newNodes[asn] = true
			}
		}
		for i, asn := range r.path.ASPath {
			if i > 0 && r.path.ASPath[i-1] != asn {
				key := [2]uint32{r.path.ASPath[i-1], asn}
				if !edges[key] {
					newEdges[key] = true
				}
			}
		}
		if !valid {
			t.Truncated = true
			continue
		}
		if len(t.Paths) >= 1000 || len(nodes)+len(newNodes) > 300 || len(edges)+len(newEdges) > 1500 {
			t.Truncated = true
			continue
		}
		t.Paths = append(t.Paths, r.path)
		origins[r.path.ASPath[len(r.path.ASPath)-1]] = true
		for i, asn := range r.path.ASPath {
			nodes[asn] = true
			if i > 0 && r.path.ASPath[i-1] != asn {
				edges[[2]uint32{r.path.ASPath[i-1], asn}] = true
			}
		}
	}
	for asn := range nodes {
		t.Nodes = append(t.Nodes, BGPNode{ASN: asn})
	}
	for asn := range origins {
		t.Origins = append(t.Origins, asn)
	}
	for edge := range edges {
		t.Edges = append(t.Edges, BGPEdge{From: edge[0], To: edge[1], Relationship: "observed"})
	}
	sort.Slice(t.Nodes, func(i, j int) bool { return t.Nodes[i].ASN < t.Nodes[j].ASN })
	sort.Slice(t.Origins, func(i, j int) bool { return t.Origins[i] < t.Origins[j] })
	sort.Slice(t.Edges, func(i, j int) bool {
		a, b := t.Edges[i], t.Edges[j]
		if origins[a.To] != origins[b.To] {
			return origins[a.To]
		}
		return a.To < b.To || a.To == b.To && a.From < b.From
	})
	if len(t.Paths) == 0 && meta.Status == "success" {
		t.Status = "empty"
	}
	return t
}

func (s *LiveService) topology(ctx context.Context, ip string) BGPTopology {
	resource := url.Values{"resource": {ip}}
	data, err := s.liveHTTP(ctx, "ripe", "/data/looking-glass/data.json", resource, nil, "")
	meta := liveMeta("ripe:looking-glass", err)
	meta.ObservedAt = textField(nested(liveObject(data), "data"), "latest_time")
	t := buildTopology(ip, ripeRows(liveObject(data), false), meta)
	if len(t.Paths) > 0 {
		return t
	}
	bits := "32"
	if addr, _ := netip.ParseAddr(ip); addr.Is6() {
		bits = "128"
	}
	data, err = s.liveHTTP(ctx, "routeviews", "/prefix/"+url.PathEscape(ip)+"/"+bits, nil, nil, "")
	t = buildTopology(ip, routeviewsRows(data), liveMeta("routeviews", err))
	if len(t.Paths) > 0 {
		return t
	}
	info, _ := s.liveHTTP(ctx, "ripe", "/data/network-info/data.json", resource, nil, "")
	hint := textField(nested(liveObject(info), "data"), "prefix")
	if hint == "" {
		hint = ip
	}
	data, err = s.liveHTTP(ctx, "ripe", "/data/bgp-state/data.json", url.Values{"resource": {hint}}, nil, "")
	meta = liveMeta("ripe:bgp-state", err)
	meta.ObservedAt = textField(nested(liveObject(data), "data"), "timestamp")
	return buildTopology(ip, ripeRows(liveObject(data), true), meta)
}
