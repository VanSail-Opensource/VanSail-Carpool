package iplookup

import (
	"context"
	"net/url"
	"strings"
)

func (s *LiveService) relationships(ctx context.Context, t *BGPTopology, c Config) {
	if len(t.Edges) == 0 {
		return
	}
	t.RelationshipMeta = LiveMeta{Status: "success", Source: "caida"}
	for _, origin := range t.Origins {
		s.originRelationships(ctx, t, origin)
	}
	for _, edge := range t.Edges {
		if edge.Relationship == "observed" {
			t.RelationshipMeta.Status = "partial"
			break
		}
	}
	key, _ := s.credential(ctx, CloudflareID, c)
	if key == "" {
		return
	}
	for _, asn := range t.Origins {
		needsFallback := false
		for _, edge := range t.Edges {
			needsFallback = needsFallback || (edge.Relationship == "observed" && (edge.To == asn || edge.From == asn))
		}
		if !needsFallback {
			continue
		}
		data, err := s.liveHTTP(ctx, CloudflareID, "/radar/entities/asns/"+asnText(asn)+"/rel", nil, nil, key)
		if err != nil {
			continue
		}
		result := nested(liveObject(data), "result")
		applied := false
		for _, r := range liveList(result, "rels") {
			for i, edge := range t.Edges {
				if edge.Relationship == "observed" && textField(r, "asn1") == asnText(edge.To) && textField(r, "asn2") == asnText(edge.From) {
					t.Edges[i].Relationship = relationship(textField(r, "rel"))
					applied = applied || t.Edges[i].Relationship != "observed"
				}
			}
		}
		if applied && t.RelationshipMeta.Status != "success" {
			date := t.RelationshipMeta.ObservedAt
			if date == "" {
				date = textField(nested(result, "meta"), "data_time")
			}
			t.RelationshipMeta = LiveMeta{Status: "partial", Source: "caida,cloudflare", ObservedAt: date}
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
