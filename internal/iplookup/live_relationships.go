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
	if len(t.Paths) > 0 {
		*t = CompactTopology(*t)
	}
	t.RelationshipMeta = LiveMeta{Status: "success", Source: "caida"}
	applyTier1PeerEvidence(t)
	targets := relationshipTargets(*t)
	for _, origin := range targets {
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
		*t = pruneTransitContext(*t)
		return
	}
	for _, asn := range targets {
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
			left, right := textField(r, "asn1"), textField(r, "asn2")
			if left != asnText(asn) && right != asnText(asn) {
				continue
			}
			for i, edge := range t.Edges {
				if edge.Relationship != "observed" {
					continue
				}
				relation := relationship(textField(r, "rel"))
				if left == asnText(edge.From) && right == asnText(edge.To) {
					relation = reverseRelationship(relation)
				} else if left != asnText(edge.To) || right != asnText(edge.From) {
					continue
				}
				if relation == "observed" {
					continue
				}
				t.Edges[i].Relationship = relation
				t.Edges[i].RelationshipMeta = &LiveMeta{Status: "success", Source: "cloudflare", ObservedAt: textField(nested(result, "meta"), "data_time")}
				applied = true
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
	*t = pruneTransitContext(*t)
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
