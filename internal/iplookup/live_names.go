package iplookup

import (
	"context"
	"net/url"
	"strconv"
	"time"
)

func (s *LiveService) cachedName(asn uint32) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.names[asn]
	if !ok || !s.now().Before(entry.expires) {
		delete(s.names, asn)
		return ""
	}
	entry.used = s.now()
	s.names[asn] = entry
	return entry.name
}

func (s *LiveService) rememberName(asn uint32, name string) {
	if name == "" || len(name) > 256 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.names[asn]; !exists && len(s.names) >= 4096 {
		var oldest uint32
		var at time.Time
		for key, v := range s.names {
			if at.IsZero() || v.used.Before(at) {
				oldest, at = key, v.used
			}
		}
		delete(s.names, oldest)
	}
	now := s.now()
	s.names[asn] = nameEntry{name: name, expires: now.Add(7 * 24 * time.Hour), used: now}
}

func asnText(asn uint32) string { return strconv.FormatUint(uint64(asn), 10) }

func (s *LiveService) resolveNames(ctx context.Context, t *BGPTopology, c Config) {
	key, _ := s.credential(ctx, "ipapi", c)
	for i := range t.Nodes {
		t.Nodes[i].Name = s.cachedName(t.Nodes[i].ASN)
	}
	for start := 0; start < len(t.Nodes); start += 100 {
		end := min(start+100, len(t.Nodes))
		queries := []string{}
		for _, node := range t.Nodes[start:end] {
			if node.Name == "" && (node.Tier1 == nil || !*node.Tier1) {
				queries = append(queries, "AS"+asnText(node.ASN))
			}
		}
		if len(queries) == 0 {
			continue
		}
		if key != "" {
			data, err := s.liveHTTP(ctx, "ipapi", "/", nil, map[string]any{"ips": queries, "key": key}, "")
			if err == nil {
				o := liveObject(data)
				for _, node := range t.Nodes[start:end] {
					d := nested(o, "AS"+asnText(node.ASN))
					s.rememberName(node.ASN, textField(d, "org"))
					if s.cachedName(node.ASN) == "" {
						s.rememberName(node.ASN, textField(d, "descr"))
					}
				}
			}
		}
		missing := []string{}
		for _, node := range t.Nodes[start:end] {
			if s.cachedName(node.ASN) == "" && (node.Tier1 == nil || !*node.Tier1) {
				missing = append(missing, asnText(node.ASN))
			}
		}
		if len(missing) > 0 {
			joined := ""
			for _, id := range missing {
				if joined != "" {
					joined += ","
				}
				joined += id
			}
			data, err := s.liveHTTP(ctx, "bgpkit", "/v3/utils/asn", url.Values{"asn": {joined}}, nil, "")
			if err == nil {
				for _, d := range liveList(liveObject(data), "data") {
					id, e := strconv.ParseUint(textField(d, "asn"), 10, 32)
					if e == nil {
						name := textField(d, "name")
						if name == "" {
							name = textField(d, "as_name")
						}
						s.rememberName(uint32(id), name)
					}
				}
			}
		}
	}
	// Resolve visible origins/direct neighbours first; avoid an unbounded per-node fanout.
	priority := map[uint32]bool{}
	for _, asn := range t.Origins {
		priority[asn] = true
	}
	for _, edge := range t.Edges {
		for _, asn := range t.Origins {
			if edge.To == asn {
				priority[edge.From] = true
			}
		}
	}
	count := 0
	for i, node := range t.Nodes {
		name := s.cachedName(node.ASN)
		if name == "" && (node.Tier1 == nil || !*node.Tier1) && priority[node.ASN] && count < 12 && ctx.Err() == nil {
			count++
			data, err := s.liveHTTP(ctx, "ripe", "/data/as-overview/data.json", url.Values{"resource": {"AS" + asnText(node.ASN)}}, nil, "")
			if err == nil {
				name = textField(nested(liveObject(data), "data"), "holder")
				s.rememberName(node.ASN, name)
			}
		}
		t.Nodes[i].Name = name
	}
}
