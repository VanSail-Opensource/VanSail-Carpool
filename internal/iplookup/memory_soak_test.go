package iplookup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestUSDTProxyMemorySoak(t *testing.T) {
	if os.Getenv("VANSAIL_MEMORY_SOAK") != "1" {
		t.Skip("hosted 256 MiB container gate")
	}
	root := os.Getenv("TMPDIR")
	if root == "" {
		t.Fatal("soak requires writable bounded-deployment storage")
	}
	s := NewEnhancedService(nil, nil, nil)
	if err := s.ConfigureInventoryCache(context.Background(), filepath.Join(root, "proxy-soak"), 1<<30); err != nil {
		t.Fatal(err)
	}
	defer s.CloseInventoryCache()
	server := soakRPCServer(t)
	deadline := time.Now().Add(3 * time.Minute)
	var previous *prefixIndex
	defer func() { previous.release() }()
	var baseline uint64
	baselineGoroutines := 0
	for cycle := 0; cycle < 8 || time.Now().Before(deadline); cycle++ {
		repo, stopScanners := startSoakScanners(t, server)
		var readers sync.WaitGroup
		stopReaders := make(chan struct{})
		failures := make(chan error, 1)
		if previous != nil {
			snapshot := previous.retain()
			readers.Add(1)
			go func() {
				defer readers.Done()
				defer snapshot.release()
				ticker := time.NewTicker(10 * time.Millisecond)
				defer ticker.Stop()
				for {
					select {
					case <-stopReaders:
						return
					case <-ticker.C:
						assessment, err := (&proxyPool{indexes: []*prefixIndex{snapshot}, sources: []ProxySourceResult{{ID: "soak"}}, complete: true}).assessmentContext(context.Background(), "11.0.0.1")
						if err != nil || len(assessment.Matches) != 1 {
							select {
							case failures <- fmt.Errorf("snapshot lost positive: %v", err):
							default:
							}
							return
						}
					}
				}
			}()
		}
		v4, v6 := 200000, 30000
		if cycle == 0 {
			v4, v6 = 3000000, 250000
		}
		s.inventoryStream = func(ctx context.Context, _ string, _ Config, emit func(string)) error {
			for i := 0; i < v4; i++ {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				emit(fmt.Sprintf("11.%d.%d.%d", i>>16, (i>>8)&255, i&255))
			}
			for i := 0; i < v6; i++ {
				emit(fmt.Sprintf("2001:ac8:%x:%x::/64", i>>16, i&65535))
			}
			return nil
		}
		index, result := s.collectSource(context.Background(), ProxySource{ID: "soak", Name: "Soak"}, DefaultConfig(), true, nil)
		close(stopReaders)
		readers.Wait()
		// Allow startup proof batches to finish even on very fast small refreshes.
		for until := time.Now().Add(20 * time.Second); repo.verifiedNetworks() < 6 && time.Now().Before(until); {
			time.Sleep(50 * time.Millisecond)
		}
		stopScanners()
		select {
		case err := <-failures:
			t.Fatal(err)
		default:
		}
		if result.Status != "success" || index.count() != v4+v6 {
			index.release()
			t.Fatalf("cycle %d: %+v", cycle, result)
		}
		if repo.verifiedNetworks() != 6 {
			t.Fatalf("verified USDT networks=%d, want all six EVM networks", repo.verifiedNetworks())
		}
		previous.release()
		previous = index
		if err := s.disk.prune(context.Background()); err != nil {
			t.Fatal(err)
		}
		server.CloseClientConnections()
		runtime.GC()
		time.Sleep(50 * time.Millisecond)
		var memory runtime.MemStats
		runtime.ReadMemStats(&memory)
		goroutines := runtime.NumGoroutine()
		peak := soakPeakRSS(t)
		if cycle == 2 {
			baseline, baselineGoroutines = memory.HeapAlloc, goroutines
		}
		if cycle > 2 && memory.HeapAlloc > baseline+(8<<20) {
			t.Fatalf("retained heap grew: baseline=%d now=%d", baseline, memory.HeapAlloc)
		}
		if cycle > 2 && goroutines > baselineGoroutines+4 {
			t.Fatalf("goroutines grew: baseline=%d now=%d", baselineGoroutines, goroutines)
		}
		if peak >= 224<<20 {
			t.Fatalf("peak RSS=%d exceeds deployment target", peak)
		}
		if usage := s.disk.usage(); usage > s.disk.quota {
			t.Fatalf("cache=%d exceeds allowance", usage)
		}
		t.Logf("cycle=%d entries=%d proof_batches=%d heap_bytes=%d goroutines=%d peak_rss_bytes=%d cache_bytes=%d", cycle, index.count(), repo.batches.Load(), memory.HeapAlloc, goroutines, peak, s.disk.usage())
	}
}

func soakPeakRSS(t *testing.T) int64 {
	t.Helper()
	raw, err := os.ReadFile("/proc/self/status")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 1 && fields[0] == "VmHWM:" {
			kib, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			return kib * 1024
		}
	}
	t.Fatal("kernel peak RSS unavailable")
	return 0
}
