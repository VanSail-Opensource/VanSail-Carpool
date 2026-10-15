package iplookup

import (
	"context"
	"fmt"
	"net/netip"
	"testing"
)

func diskService(t *testing.T, quota int64) *EnhancedService {
	t.Helper()
	s := NewEnhancedService(nil, nil, nil)
	if err := s.ConfigureInventoryCache(context.Background(), t.TempDir(), quota); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.CloseInventoryCache(); err != nil {
			t.Error(err)
		}
	})
	return s
}

func collectDiskFixture(t *testing.T, s *EnhancedService, input string) *prefixIndex {
	t.Helper()
	s.inventoryFetch = func(context.Context, string, Config) ([]byte, error) { return []byte(input), nil }
	index, result := s.collectSource(context.Background(), ProxySource{ID: "fixture", Name: "Fixture"}, DefaultConfig(), false, nil)
	if result.Status != "success" {
		t.Fatalf("collection=%+v", result)
	}
	return index
}

func TestDiskInventoryContainmentAndCounts(t *testing.T) {
	s := diskService(t, 64<<20)
	first := collectDiskFixture(t, s, "11.0.0.0/24\n2001:ac8:16::/48\n11.0.0.1")
	defer first.release()
	second := collectDiskFixture(t, s, "11.0.0.0/24\n12.0.0.1")
	defer second.release()
	for _, tc := range []struct {
		ip   string
		want bool
	}{{"11.0.0.255", true}, {"11.0.1.0", false}, {"2001:ac8:16:ffff::1", true}, {"2001:ac8:17::1", false}} {
		found, err := first.containsContext(context.Background(), netip.MustParseAddr(tc.ip))
		if err != nil || found != tc.want {
			t.Fatalf("%s found=%v err=%v", tc.ip, found, err)
		}
	}
	count, err := countPrefixIndexesContext(context.Background(), []*prefixIndex{first, second})
	if err != nil || count != 4 {
		t.Fatalf("unique=%d err=%v", count, err)
	}
	merged := mergePrefixIndexes(first, second, 10)
	defer merged.release()
	if merged.count() != 4 {
		t.Fatalf("merged=%d", merged.count())
	}
	found, err := merged.containsContext(context.Background(), netip.MustParseAddr("12.0.0.1"))
	if err != nil || !found {
		t.Fatalf("merge lost incoming evidence: %v", err)
	}
}

func TestDiskInventoryRetiresOnlyUnreferencedGenerations(t *testing.T) {
	s := diskService(t, 32<<20)
	index := collectDiskFixture(t, s, "11.0.0.0/24")
	snapshot := index.retain()
	index.release()
	if err := s.disk.prune(context.Background()); err != nil {
		t.Fatal(err)
	}
	if found, err := snapshot.containsContext(context.Background(), netip.MustParseAddr("11.0.0.1")); err != nil || !found {
		t.Fatal("reader lost retired source", err)
	}
	snapshot.release()
	if err := s.disk.prune(context.Background()); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := s.disk.db.QueryRow("SELECT count(*) FROM prefixes").Scan(&count); err != nil || count != 0 {
		t.Fatalf("retired rows=%d err=%v", count, err)
	}
}

func TestDiskInventoryQuotaPreservesPartialEvidence(t *testing.T) {
	s := diskService(t, 16<<20)
	// Exercise SQLite's page ceiling without allocating a giant input document.
	s.inventoryStream = func(ctx context.Context, _ string, _ Config, emit func(string)) error {
		for i := 0; i < 300000; i++ {
			emit(fmt.Sprintf("2001:ac8:%x:%x::/64", i>>16, i&65535))
		}
		return nil
	}
	index, result := s.collectSource(context.Background(), ProxySource{ID: "quota"}, DefaultConfig(), false, nil)
	defer index.release()
	if result.Status != "partial" || result.ErrorCode != "IP_PROXY_LIMIT" || index.count() == 0 {
		t.Fatalf("quota=%+v", result)
	}
	if size := s.disk.usage(); size > s.disk.quota {
		t.Fatalf("disk=%d quota=%d", size, s.disk.quota)
	}
	found, err := index.containsContext(context.Background(), netip.MustParseAddr("2001:ac8::1"))
	if err != nil || !found {
		t.Fatalf("lost committed positive: %v", err)
	}
}

func TestDiskInventoryReadFailureCannotCertifyMiss(t *testing.T) {
	s := diskService(t, 32<<20)
	index := collectDiskFixture(t, s, "11.0.0.1")
	defer index.release()
	if _, err := s.disk.db.Exec("DROP TABLE prefixes"); err != nil {
		t.Fatal(err)
	}
	pool := &proxyPool{indexes: []*prefixIndex{index}, sources: []ProxySourceResult{{ID: "fixture"}}, complete: true}
	assessment, err := pool.assessmentContext(context.Background(), "12.0.0.1")
	if err == nil || assessment.Status != "partial" {
		t.Fatal("storage failure certified a negative")
	}
}
