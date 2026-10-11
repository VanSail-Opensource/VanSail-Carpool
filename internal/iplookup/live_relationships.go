package iplookup

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

func (s *LiveService) relationships(ctx context.Context, t *BGPTopology, c Config) {
	if len(t.Edges) == 0 {
		return
	}
	// Exact-pair queries avoid downloading unrelated customer cones.
	limit := min(len(t.Edges), 200)
	t.RelationshipMeta = LiveMeta{Status: "success", Source: "caida"}
	for start := 0; start < limit; start += 50 {
		var q strings.Builder
		q.WriteString("{ dataset { date } ")
		end := min(start+50, limit)
		for i := start; i < end; i++ {
			edge := t.Edges[i]
			fmt.Fprintf(&q, "e%d:asnLink(asn0:\"%d\",asn1:\"%d\"){relationship date asn0{asn asnName} asn1{asn asnName}} ", i, edge.To, edge.From)
		}
		q.WriteString("}")
		data, err := s.liveHTTP(ctx, "caida", "/v2/graphql", nil, map[string]string{"query": q.String()}, "")
		o := liveObject(data)
		if err != nil || graphqlFailed(o) {
			if err == nil {
				err = &CodeError{"IP_DETAILS_INVALID_RESPONSE"}
			}
			t.RelationshipMeta = liveMeta("caida", err)
			t.RelationshipMeta.Status = "partial"
			break
		}
		d := nested(o, "data")
		t.RelationshipMeta.ObservedAt = textField(nested(d, "dataset"), "date")
		for i := start; i < end; i++ {
			link := nested(d, fmt.Sprintf("e%d", i))
			t.Edges[i].Relationship = relationship(textField(link, "relationship"))
			s.rememberName(t.Edges[i].To, textField(nested(link, "asn0"), "asnName"))
			s.rememberName(t.Edges[i].From, textField(nested(link, "asn1"), "asnName"))
		}
	}
	if limit < len(t.Edges) {
		t.RelationshipMeta.Status = "partial"
	}
	key, _ := s.credential(ctx, CloudflareID, c)
	if key == "" {
		return
	}
	for _, asn := range t.Origins {
		data, err := s.liveHTTP(ctx, CloudflareID, "/radar/entities/asns/"+asnText(asn)+"/rel", nil, nil, key)
		if err != nil {
			continue
		}
		result := nested(liveObject(data), "result")
		for _, r := range liveList(result, "rels") {
			for i, edge := range t.Edges {
				if edge.Relationship == "observed" && textField(r, "asn1") == asnText(edge.To) && textField(r, "asn2") == asnText(edge.From) {
					t.Edges[i].Relationship = relationship(textField(r, "rel"))
				}
			}
		}
		if t.RelationshipMeta.Status != "success" {
			t.RelationshipMeta = LiveMeta{Status: "partial", Source: "caida,cloudflare", ObservedAt: textField(nested(result, "meta"), "data_time")}
		}
	}
}

func relationship(value string) string {
	switch strings.ToLower(value) {
	case "provider", "c2p":
		return "upstream"
	case "peer", "p2p":
		return "peer"
	case "customer", "p2c":
		return "customer"
	default:
		return "observed"
	}
}

func ripeQuery(asn uint32) url.Values { return url.Values{"resource": {"AS" + asnText(asn)}} }
