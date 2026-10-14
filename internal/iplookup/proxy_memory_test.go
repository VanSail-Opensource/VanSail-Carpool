package iplookup

import (
	"context"
	"encoding/binary"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RoaringBitmap/roaring/v2"
)

func TestIPv4CIDROrdinalsCompressAdjacentNetworks(t *testing.T) {
	b := newPrefixIndexBuilder(0)
	previous := roaring.New()
	for i := range 65536 {
		n := uint32(0x0b000000) + uint32(i)*256
		var raw [4]byte
		binary.BigEndian.PutUint32(raw[:], n)
		b.add(netip.PrefixFrom(netip.AddrFrom4(raw), 24))
		previous.Add(n)
	}
	previous.RunOptimize()
	index := b.freeze()
	compressed, old := index.v4[24].GetSizeInBytes(), previous.GetSizeInBytes()
	if index.count() != 65536 || compressed >= old/10 {
		t.Fatalf("count=%d compressed=%d previous=%d", index.count(), compressed, old)
	}
	for _, ip := range []string{"11.0.0.0", "11.127.42.99", "11.255.255.255"} {
		if !index.contains(netip.MustParseAddr(ip)) {
			t.Fatalf("lost CIDR membership: %s", ip)
		}
	}
	if index.contains(netip.MustParseAddr("12.0.0.1")) {
		t.Fatal("expanded outside the listed prefixes")
	}
	t.Logf("entries=65536 compressed_bytes=%d previous_bytes=%d", compressed, old)
}

func TestStreamingSourceCountsKeepPrefixIdentity(t *testing.T) {
	indexes := []*prefixIndex{
		testPrefixIndex("0.0.0.0/0", "11.0.0.0/24", "11.0.0.1/32", "2001:db8::/32"),
		testPrefixIndex("11.0.0.0/24", "11.0.0.1/32", "2001:db8::/32", "2001:db8::1/128"),
		testPrefixIndex("11.0.0.2/32", "2001:db8::1/128", "2001:db8::2/128"),
		testPrefixIndex("11.0.0.2/32", "255.255.255.255/32", "::/0"), nil,
	}
	var combined *prefixIndex
	for _, index := range indexes {
		combined = mergePrefixIndexes(combined, index, 0)
	}
	if got := countPrefixIndexes(indexes); got != 9 || got != combined.count() {
		t.Fatalf("streamed count=%d, union=%d", got, combined.count())
	}
	if countPrefixIndexes(nil) != 0 || countPrefixIndexes([]*prefixIndex{indexes[0]}) != 4 {
		t.Fatal("empty/single-source count changed")
	}
}

func TestExpiredSourceIndexesAreRetiredBeforeRefresh(t *testing.T) {
	s := NewEnhancedService(nil, nil, nil)
	c := DefaultConfig()
	c.ProxySources = []ProxySource{{ID: "enabled", Enabled: true}, {ID: "disabled"}}
	b := &sourceTestRuntime{ctx: context.Background(), entries: map[string]sourceTestEntry{}, sources: map[string]*sourceCache{}}
	s.tests = b
	for _, source := range c.ProxySources {
		b.sources[source.ID] = &sourceCache{source: source, key: sourceCacheKey(source, c), index: testPrefixIndex("11.0.0.0/24"), complete: true,
			expires: time.Now().Add(-time.Second), nextRefresh: time.Now().Add(time.Hour)}
	}
	s.reconcileSourcesLocked(b, c)
	b.refreshCount()
	for _, cache := range b.sources {
		if cache.index != nil || cache.complete || cache.result.Count != 0 {
			t.Fatal("expired enabled/disabled inventory retained")
		}
	}
	pool, err := s.cachedPool(context.Background(), c)
	if err != nil || pool.complete || b.uniqueEntries != 0 {
		t.Fatal("retired evidence certified a negative result")
	}
}

func TestInventoryMaintenanceReleasesOnlyExpiredMemos(t *testing.T) {
	s := NewEnhancedService(nil, nil, nil)
	expired, fresh := time.Now().Add(-time.Second), time.Now().Add(time.Hour)
	s.dns["expired"], s.dns["fresh"] = dnsEntry{expires: expired}, dnsEntry{expires: fresh}
	s.pools[[32]byte{1}], s.pools[[32]byte{2}] = &proxyPool{expires: expired}, &proxyPool{expires: fresh}
	s.pruneInventoryMemos()
	if len(s.dns) != 1 || len(s.pools) != 1 || s.dns["fresh"].expires != fresh || s.pools[[32]byte{2}] == nil {
		t.Fatal("cleanup retained expired memos or evicted fresh ones")
	}
}

func TestSourceRefreshCollectionsAreSerial(t *testing.T) {
	s := diagnosticService(t)
	c := enabledSourceConfig()
	c.ProxySources = []ProxySource{{ID: "one", Name: "One", Enabled: true, URL: "https://example.net/one"}, {ID: "two", Name: "Two", Enabled: true, URL: "https://example.net/two"}}
	settings := &sourceSettings{}
	settings.set(t, c)
	s.settings = settings
	var active, peak atomic.Int32
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	s.inventoryFetch = func(ctx context.Context, _ string, _ Config) ([]byte, error) {
		current := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); current > old && !peak.CompareAndSwap(old, current); old = peak.Load() {
		}
		entered <- struct{}{}
		select {
		case <-release:
			return []byte("11.0.0.0/24"), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.StartSourceTests(ctx)
	t.Cleanup(func() { cancel(); s.WaitSourceTests() })
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("startup did not collect")
	}
	select {
	case <-entered:
		t.Fatal("two source collections overlapped")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	awaitSource(t, s, "one")
	awaitSource(t, s, "two")
	if peak.Load() != 1 {
		t.Fatal("source refresh admitted parallel collectors")
	}
}
