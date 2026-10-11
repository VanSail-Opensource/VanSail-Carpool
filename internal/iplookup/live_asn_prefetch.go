package iplookup

import (
	"context"
	"net/url"
	"sync"
)

func asnMetadataQuery(id uint32) map[string]string {
	return map[string]string{"query": "{dataset{date} asn(asn:\"" + asnText(id) + "\"){asnName source asnDegree{peer provider}}}"}
}

// prefetchASN overlaps independent source calls; the lookup-local memo delivers them once.
func (s *LiveService) prefetchASN(ctx context.Context, id uint32) {
	if _, ok := ctx.Value(liveRequestsKey{}).(*liveRequests); !ok {
		return
	}
	var work sync.WaitGroup
	work.Add(3)
	go func() {
		defer work.Done()
		q := ripeQuery(id)
		q.Set("lod", "2")
		_, _ = s.liveHTTP(ctx, "ripe", "/data/rir/data.json", q, nil, "")
	}()
	go func() {
		defer work.Done()
		_, _ = s.liveHTTP(ctx, "ripe", "/data/routing-status/data.json", ripeQuery(id), nil, "")
	}()
	go func() {
		defer work.Done()
		_, _ = s.liveHTTP(ctx, "caida", "/v2/graphql", url.Values{}, asnMetadataQuery(id), "")
	}()
	work.Wait()
}
