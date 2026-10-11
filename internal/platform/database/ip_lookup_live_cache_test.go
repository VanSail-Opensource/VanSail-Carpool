package database

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/iplookup"
)

func TestPersistentLiveCacheMatchingRestartAndCompactStorage(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, _, _, _, now := ipLookupFixture(t, 1)
	r := iplookup.LiveDetails{IP: "1.1.1.10", FetchedAt: now.Format(time.RFC3339Nano), Topology: iplookup.BGPTopology{Origins: []uint32{10}, Nodes: []iplookup.BGPNode{{ASN: 10}, {ASN: 20}, {ASN: 30}}, Edges: []iplookup.BGPEdge{{From: 20, To: 10}, {From: 30, To: 20}}, Paths: []iplookup.BGPPath{{ASPath: []uint32{30, 20, 10}}}}}
	id, err := store.SaveIPDetails(ctx, "", r, now)
	if err != nil {
		t.Fatal(err)
	}
	restarted := NewStore(store.DB())
	for _, tc := range []struct{ ip, match string }{{"1.1.1.10", "exact"}, {"1.1.1.11", "subnet"}, {"1.1.2.10", "none"}} {
		cached, err := restarted.CachedIPDetails(ctx, tc.ip)
		if err != nil {
			t.Fatal(err)
		}
		if tc.match == "none" {
			if cached != nil {
				t.Fatal("wrong subnet")
			}
			continue
		}
		if cached == nil || cached.IP != tc.ip || cached.SourceIP != r.IP || cached.CacheMatch != tc.match || cached.SnapshotID != id || len(cached.Topology.Paths) != 0 || len(cached.Topology.Nodes) != 3 {
			t.Fatalf("invalid cache: %+v", cached)
		}
	}
	var raw string
	if err := store.DB().QueryRowContext(ctx, `SELECT details_json FROM ip_lookup_live_snapshots WHERE id=?`, id).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var saved iplookup.LiveDetails
	if err := json.Unmarshal([]byte(raw), &saved); err != nil || len(saved.Topology.Paths) != 0 {
		t.Fatal("stored original paths")
	}
	r.IP = "2606:4700:4700::1111"
	if _, err := store.SaveIPDetails(ctx, "", r, now); err != nil {
		t.Fatal(err)
	}
	if cached, err := restarted.CachedIPDetails(ctx, "2606:4700:4700::1001"); err != nil || cached != nil {
		t.Fatal("IPv6 subnet reuse")
	}
}

func TestLiveCacheOrdersFullTimestampThenDurableRowID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	base := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)
	for i, offset := range []time.Duration{900, 100, 900} {
		r := iplookup.LiveDetails{IP: "8.8.8.8", FetchedAt: base.Add(time.Duration(i) * time.Second).Format(time.RFC3339Nano)}
		if _, err := store.SaveIPDetails(ctx, "", r, base.Add(offset)); err != nil {
			t.Fatal(err)
		}
	}
	r, err := store.CachedIPDetails(ctx, "8.8.8.9")
	if err != nil || r == nil || r.FetchedAt != base.Add(2*time.Second).Format(time.RFC3339Nano) {
		t.Fatalf("wrong newest result: %+v %v", r, err)
	}
}
