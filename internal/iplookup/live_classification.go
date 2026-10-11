package iplookup

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// classify uses CAIDA's explicit cliqueMember field, never a degree threshold.
func (s *LiveService) classify(ctx context.Context, t *BGPTopology) {
	if len(t.Nodes) == 0 {
		return
	}
	t.ClassificationMeta = LiveMeta{Status: "success", Source: "caida:clique"}
	for start := 0; start < len(t.Nodes); start += 50 {
		var query strings.Builder
		query.WriteString("{dataset{date} ")
		end := min(start+50, len(t.Nodes))
		for i := start; i < end; i++ {
			fmt.Fprintf(&query, "n%d:asn(asn:\"%d\"){asnName cliqueMember} ", i, t.Nodes[i].ASN)
		}
		query.WriteString("}")
		data, err := s.liveHTTP(ctx, "caida", "/v2/graphql", nil, map[string]string{"query": query.String()}, "")
		o := liveObject(data)
		if err != nil || graphqlFailed(o) {
			if err == nil {
				err = &CodeError{"IP_DETAILS_INVALID_RESPONSE"}
			}
			observedAt := t.ClassificationMeta.ObservedAt
			t.ClassificationMeta = liveMeta("caida:clique", err)
			t.ClassificationMeta.ObservedAt = observedAt
			return
		}
		d := nested(o, "data")
		t.ClassificationMeta.ObservedAt = textField(nested(d, "dataset"), "date")
		for i := start; i < end; i++ {
			asn := nested(d, fmt.Sprintf("n%d", i))
			t.Nodes[i].Tier1 = flag(asn, "cliqueMember")
			if t.Nodes[i].Tier1 == nil {
				if v, err := strconv.ParseBool(textField(asn, "cliqueMember")); err == nil {
					t.Nodes[i].Tier1 = &v
				}
			}
			if t.Nodes[i].Tier1 == nil {
				t.ClassificationMeta.Status = "partial"
			}
			s.rememberName(t.Nodes[i].ASN, textField(asn, "asnName"))
			t.Nodes[i].Name = s.cachedName(t.Nodes[i].ASN)
		}
	}
}

func graphqlFailed(o object) bool {
	if len(o["errors"]) == 0 || string(o["errors"]) == "null" {
		return false
	}
	var failures []json.RawMessage
	return json.Unmarshal(o["errors"], &failures) != nil || len(failures) > 0
}
