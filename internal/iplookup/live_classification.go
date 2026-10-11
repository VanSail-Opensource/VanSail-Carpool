package iplookup

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// classify uses the dated dataset clique. The per-ASN cliqueMember API flag
// incorrectly reports true for non-members and must never drive recognition.
func (s *LiveService) classify(ctx context.Context, t *BGPTopology) {
	if len(t.Nodes) == 0 {
		return
	}
	ids := make([]string, len(t.Nodes))
	for i := range t.Nodes {
		t.Nodes[i].Tier1 = nil
		ids[i] = fmt.Sprintf("\"%d\"", t.Nodes[i].ASN)
	}
	query := fmt.Sprintf("{dataset{date clique} asns(asns:[%s],first:%d){edges{node{asn asnName}}}}", strings.Join(ids, ","), len(ids))
	data, err := s.liveHTTP(ctx, "caida", "/v2/graphql", nil, map[string]string{"query": query}, "")
	t.ClassificationMeta = liveMeta("caida:dataset-clique", err)
	if err != nil {
		return
	}
	o := liveObject(data)
	d := nested(o, "data")
	dataset := nested(d, "dataset")
	date := textField(dataset, "date")
	var members []string
	if date == "" || json.Unmarshal(dataset["clique"], &members) != nil || len(members) == 0 || len(members) > 128 {
		t.ClassificationMeta = liveMeta("caida:dataset-clique", &CodeError{"IP_DETAILS_INVALID_RESPONSE"})
		return
	}
	clique := make(map[uint32]bool, len(members))
	for _, member := range members {
		asn, err := strconv.ParseUint(member, 10, 32)
		if err != nil || asn == 0 {
			t.ClassificationMeta = liveMeta("caida:dataset-clique", &CodeError{"IP_DETAILS_INVALID_RESPONSE"})
			return
		}
		clique[uint32(asn)] = true
	}
	for _, edge := range liveList(nested(d, "asns"), "edges") {
		node := nested(edge, "node")
		if asn, err := strconv.ParseUint(textField(node, "asn"), 10, 32); err == nil {
			s.rememberName(uint32(asn), textField(node, "asnName"))
		}
	}
	t.ClassificationMeta.ObservedAt = date
	if graphqlFailed(o) {
		t.ClassificationMeta.Status, t.ClassificationMeta.ErrorCode = "partial", "IP_DETAILS_INVALID_RESPONSE"
	}
	for i := range t.Nodes {
		member := clique[t.Nodes[i].ASN]
		t.Nodes[i].Tier1 = &member
		t.Nodes[i].Name = s.cachedName(t.Nodes[i].ASN)
	}
}

func graphqlFailed(o object) bool {
	if len(o["errors"]) == 0 || string(o["errors"]) == "null" {
		return false
	}
	var failures []json.RawMessage
	return json.Unmarshal(o["errors"], &failures) != nil || len(failures) > 0
}
