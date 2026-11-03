package xnode

import (
	"fmt"
	"os"
	"runtime"
	"runtime/metrics"
	"testing"
	"time"
)

func automaticLeaseFixture(count int, at time.Time) []AutomaticLeaseObservation {
	items := make([]AutomaticLeaseObservation, count)
	for i := range items {
		items[i] = AutomaticLeaseObservation{fmt.Sprintf("8.1.%d.%d", i>>8, i&255), "node", at}
	}
	return items
}

func TestAutomaticLeases10000Performance(t *testing.T) {
	if os.Getenv("XNODE_LEASE_PERF_GATE") != "1" {
		t.Skip("timing and retained-memory gate runs in its isolated hosted invocation")
	}
	at := time.Now().UTC()
	observations := automaticLeaseFixture(10000, at)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	cpu := []metrics.Sample{{Name: "/cpu/classes/user:cpu-seconds"}}
	metrics.Read(cpu)
	cpuBefore := cpu[0].Value.Float64()
	m := newAutomaticLeaseManager("performance")
	pass := func(name string, run func()) {
		t.Helper()
		start := time.Now()
		run()
		elapsed := time.Since(start)
		t.Logf("phase=%s ips=10000 latency_ms=%.3f", name, float64(elapsed)/float64(time.Millisecond))
		if elapsed > time.Second {
			t.Fatalf("registry pass exceeded one second: %s %s", name, elapsed)
		}
	}
	pass("creation", func() { m.observe(observations, at, time.Minute) })
	var cursor uint64
	pass("node_sync", func() { cursor = verifyAutomaticLeaseDelivery(t, m, 0, at, 10000) })
	writes := drainAutomaticLeaseBans(m, at)
	pass("repeated_observations", func() {
		for i := range observations {
			observations[i].At = at.Add(time.Hour)
		}
		for range 10 {
			m.observe(observations, at.Add(time.Hour), time.Minute)
		}
	})
	if extra := drainAutomaticLeaseBans(m, at.Add(time.Hour)); extra != 0 {
		t.Fatal("steady observations requested database operations", extra)
	}
	if grants, _, _ := m.delivery(cursor, at.Add(time.Hour)); len(grants) != 0 {
		t.Fatal("unchanged lease produced duplicate grants")
	}
	runtime.GC()
	runtime.ReadMemStats(&after)
	retained := max(0, int64(after.HeapAlloc)-int64(before.HeapAlloc))
	if retained >= 64<<20 {
		t.Fatal("lease manager retained at least 64 MiB", retained)
	}
	pass("reconnect_sync", func() { verifyAutomaticLeaseDelivery(t, m, 0, at.Add(time.Hour), 10000) })
	pass("renewal", func() {
		for i := range observations {
			observations[i].At = at.Add(12 * time.Hour)
		}
		m.observe(observations, at.Add(12*time.Hour), time.Minute)
	})
	pass("renewed_sync", func() { verifyAutomaticLeaseDelivery(t, m, cursor, at.Add(12*time.Hour), 10000) })
	pass("expiry", func() { m.expire(at.Add(60 * time.Hour)) })
	if len(m.leases)+len(m.expiry)+len(m.snapshot)+len(m.pendingBans) != 0 {
		t.Fatal("expiry retained lease IPs or removal delivery")
	}
	metrics.Read(cpu)
	t.Logf("retained_bytes=%d allocations=%d runtime_user_cpu_seconds=%.6f initial_ban_batches=%d steady_ban_batches=0 duplicate_grants=0", retained, after.Mallocs-before.Mallocs, cpu[0].Value.Float64()-cpuBefore, writes)
	runtime.KeepAlive(observations)
	runtime.KeepAlive(m)
}

func verifyAutomaticLeaseDelivery(t *testing.T, m *automaticLeaseManager, cursor uint64, now time.Time, want int) uint64 {
	t.Helper()
	seen := map[string]bool{}
	for {
		items, through, more := m.delivery(cursor, now)
		if len(items) > automaticLeaseBatch {
			t.Fatal("delivery was not batched")
		}
		for _, item := range items {
			if seen[item.IP] || item.Generation <= cursor {
				t.Fatal("duplicate grant or regressing generation", item)
			}
			seen[item.IP] = true
		}
		cursor = through
		if !more {
			break
		}
	}
	if len(seen) != want {
		t.Fatal("node did not synchronize every lease", len(seen), want)
	}
	return cursor
}

func drainAutomaticLeaseBans(m *automaticLeaseManager, now time.Time) int {
	writes := 0
	for {
		items := m.pendingBanReleases(now)
		if len(items) == 0 {
			return writes
		}
		writes++
		m.acknowledgeBanReleases(items)
	}
}

func BenchmarkAutomaticLeaseRegistry10000(b *testing.B) {
	at := time.Now().UTC()
	observations := automaticLeaseFixture(10000, at)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		m := newAutomaticLeaseManager("benchmark")
		m.observe(observations, at, time.Minute)
		for _, item := range observations {
			item.At = at.Add(12 * time.Hour)
			m.observe([]AutomaticLeaseObservation{item}, item.At, time.Minute)
		}
		var cursor uint64
		for {
			_, through, more := m.delivery(cursor, at.Add(12*time.Hour))
			cursor = through
			if !more {
				break
			}
		}
		m.expire(at.Add(60 * time.Hour))
	}
	b.ReportMetric(10000, "ips/op")
}
