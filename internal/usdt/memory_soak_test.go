package usdt_test

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestUSDTMemorySoak(t *testing.T) {
	if os.Getenv("VANSAIL_MEMORY_SOAK") != "1" {
		t.Skip("hosted 256 MiB container gate")
	}
	server, waitHandlers := soakRPCServer(t)
	deadline := time.Now().Add(3 * time.Minute)
	var baseline uint64
	baselineGoroutines := 0
	for cycle := 0; cycle < 8 || time.Now().Before(deadline); cycle++ {
		repo, stop := startSoakScanners(t, server)
		for until := time.Now().Add(20 * time.Second); time.Now().Before(until); {
			time.Sleep(50 * time.Millisecond)
		}
		stop()
		if repo.verifiedNetworks() != 6 {
			t.Fatalf("verified networks=%d", repo.verifiedNetworks())
		}
		server.CloseClientConnections()
		waitHandlers()
		runtime.GC()
		var memory runtime.MemStats
		runtime.ReadMemStats(&memory)
		goroutines, peak := runtime.NumGoroutine(), soakPeakRSS(t)
		if cycle == 2 {
			baseline, baselineGoroutines = memory.HeapAlloc, goroutines
		}
		if cycle > 2 && memory.HeapAlloc > baseline+(8<<20) {
			t.Fatalf("heap grew: baseline=%d now=%d", baseline, memory.HeapAlloc)
		}
		if cycle > 2 && goroutines > baselineGoroutines+4 {
			t.Fatalf("goroutines grew: baseline=%d now=%d", baselineGoroutines, goroutines)
		}
		if peak >= 224<<20 {
			t.Fatalf("peak RSS=%d exceeds deployment target", peak)
		}
		t.Logf("cycle=%d proof_batches=%d heap_bytes=%d goroutines=%d peak_rss_bytes=%d", cycle, repo.batches.Load(), memory.HeapAlloc, goroutines, peak)
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
