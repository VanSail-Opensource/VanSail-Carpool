package iplookup

import (
	"context"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
)

func TestDiskInventoryRebuildsOnlyOwnedCacheFiles(t *testing.T) {
	dir := t.TempDir()
	keep := filepath.Join(dir, "operator-note")
	if err := os.WriteFile(keep, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	first, err := openInventoryDisk(context.Background(), dir, 32<<20)
	if err != nil {
		t.Fatal(err)
	}
	index, err := first.newGeneration(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	writer := &diskWriter{ctx: context.Background(), index: index, limit: 100}
	writer.add(netip.MustParsePrefix("11.0.0.0/24"))
	writer.flush()
	if writer.err != nil {
		t.Fatal(writer.err)
	}
	if err := first.db.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := openInventoryDisk(context.Background(), dir, 32<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer second.db.Close()
	var count int
	if err := second.db.QueryRow("SELECT count(*) FROM prefixes").Scan(&count); err != nil || count != 0 {
		t.Fatal("restart reused stale evidence", err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatal("removed unrelated file", err)
	}
}

func TestDiskInventoryStagesDomainsWithBoundedPages(t *testing.T) {
	s := diskService(t, 32<<20)
	index, err := s.disk.newGeneration(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer index.release()
	writer := &diskWriter{ctx: context.Background(), index: index, limit: 100}
	for range 300 {
		writer.addDomain("edge.example.net")
	}
	writer.flush()
	if writer.err != nil || writer.hosts != 1 {
		t.Fatalf("domain dedup=%d err=%v", writer.hosts, writer.err)
	}
	page, err := writer.domainPage("")
	if err != nil || len(page) != 1 {
		t.Fatalf("domains=%v err=%v", page, err)
	}
	if err := writer.clearDomains(); err != nil {
		t.Fatal(err)
	}
	page, err = writer.domainPage("")
	if err != nil || len(page) != 0 {
		t.Fatal("domain staging retained", err)
	}
}

func TestDiskInventoryUnavailableDoesNotFallBackToMemory(t *testing.T) {
	s := NewEnhancedService(nil, nil, nil)
	if err := s.ConfigureInventoryCache(context.Background(), t.TempDir(), 1); err == nil {
		t.Fatal("invalid quota accepted")
	}
	called := false
	s.inventoryStream = func(context.Context, string, Config, func(string)) error { called = true; return nil }
	index, result := s.collectSource(context.Background(), ProxySource{ID: "unavailable"}, DefaultConfig(), false, nil)
	if called || index != nil || result.Status == "success" {
		t.Fatal("unavailable disk cache used memory or certified coverage")
	}
}
