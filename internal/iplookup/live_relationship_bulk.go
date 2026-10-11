package iplookup

import (
	"context"
	"fmt"
	"strconv"
)

// originRelationships uses a paginated connection rather than slow per-link
// GraphQL aliases. Only already-observed edges are annotated; source rows are
// transient and cannot add nodes, paths or inferred physical connectivity.
func (s *LiveService) originRelationships(ctx context.Context, t *BGPTopology, origin uint32) {
	indices := map[[2]uint32][]int{}
	unresolved := map[int]bool{}
	for i, edge := range t.Edges {
		if edge.From != origin && edge.To != origin {
			continue
		}
		indices[[2]uint32{edge.To, edge.From}] = append(indices[[2]uint32{edge.To, edge.From}], i)
		if edge.Relationship == "observed" {
			unresolved[i] = true
		}
	}
	if len(unresolved) == 0 {
		return
	}
	offset := 0
	// Four 1,000-row pages bound large AS neighborhoods independently of the
	// topology cap. Stop as soon as every observed direct pair is classified.
	for page := 0; page < 4 && len(unresolved) > 0 && ctx.Err() == nil; page++ {
		query := fmt.Sprintf("{dataset{date} asnLinks(asn:\"%d\",first:1000,offset:%d){totalCount edges{node{asn0{asn} asn1{asn} relationship date}}}}", origin, offset)
		data, err := s.liveHTTP(ctx, "caida", "/v2/graphql", nil, map[string]string{"query": query}, "")
		if err != nil {
			t.RelationshipMeta.Status, t.RelationshipMeta.ErrorCode = "partial", ErrorCode(err)
			return
		}
		o := liveObject(data)
		d := nested(o, "data")
		date := textField(nested(d, "dataset"), "date")
		if date != "" {
			t.RelationshipMeta.ObservedAt = date
		}
		connection := nested(d, "asnLinks")
		rows := liveList(connection, "edges")
		for _, row := range rows {
			link := nested(row, "node")
			left, err0 := strconv.ParseUint(textField(nested(link, "asn0"), "asn"), 10, 32)
			right, err1 := strconv.ParseUint(textField(nested(link, "asn1"), "asn"), 10, 32)
			if err0 != nil || err1 != nil || (uint32(left) != origin && uint32(right) != origin) {
				continue
			}
			relation := relationship(textField(link, "relationship"))
			if relation == "observed" {
				continue
			}
			for _, i := range indices[[2]uint32{uint32(left), uint32(right)}] {
				t.Edges[i].Relationship = relation
				delete(unresolved, i)
			}
			for _, i := range indices[[2]uint32{uint32(right), uint32(left)}] {
				t.Edges[i].Relationship = reverseRelationship(relation)
				delete(unresolved, i)
			}
		}
		if graphqlFailed(o) {
			t.RelationshipMeta.Status, t.RelationshipMeta.ErrorCode = "partial", "IP_DETAILS_INVALID_RESPONSE"
		}
		total := integer(connection, "totalCount")
		if total == nil || date == "" {
			t.RelationshipMeta.Status, t.RelationshipMeta.ErrorCode = "partial", "IP_DETAILS_INVALID_RESPONSE"
			return
		}
		offset += len(rows)
		if len(rows) == 0 || offset >= *total {
			break
		}
	}
}

func reverseRelationship(relation string) string {
	if relation == "upstream" {
		return "customer"
	}
	if relation == "customer" {
		return "upstream"
	}
	return relation
}
