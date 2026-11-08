package xnode

import (
	"context"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestHourlyCreditWorkerUsesServiceClockForDemandAndRetry(t *testing.T) {
	m, peer, _, now := masterHourlyRateFixture(t)
	*now = time.Date(2099, time.January, 1, 0, 0, 0, 0, time.UTC)
	c := m.hourlyCredits
	c.mu.Lock()
	state := c.stateLocked(peer, *now)
	state.scanned = true
	state.hints[wire.HourlyOwner{ID: "owner", Generation: 1}] = hourlyCreditHint{seenAt: *now, demand: wire.HourlyDemand{PendingBytes: 1}}
	c.hintCount++
	c.mu.Unlock()
	ctx, cancel := context.WithCancel(peer.ctx)
	m.wg.Add(1)
	go m.runHourlyCredits(ctx)
	t.Cleanup(func() { cancel(); m.Wait() })
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		c.mu.Lock()
		visited := c.cursor > 0
		c.mu.Unlock()
		if visited {
			break
		}
		select {
		case <-timer.C:
			t.Fatal("real wall clock suppressed service-clock demand scheduling")
		case <-ticker.C:
		}
	}
	cancel()
	m.Wait()
	if peers := c.ready(m.service.now()); len(peers) != 1 {
		t.Fatal("worker retained an unexpected clock-based scheduling delay")
	}
	c.finished(peer, false, true, m.service.now())
	c.mu.Lock()
	retry := state.retryAt
	c.mu.Unlock()
	if !retry.Equal(now.Add(time.Second)) {
		t.Fatal("failure retry used a different clock from demand activity", retry)
	}
	if peers := c.ready(now.Add(time.Second - time.Nanosecond)); len(peers) != 0 {
		t.Fatal("failed credit work retried before its service-clock boundary")
	}
	if peers := c.ready(now.Add(time.Second)); len(peers) != 1 {
		t.Fatal("failed credit work missed its exact service-clock retry boundary")
	}
	c.finished(peer, false, false, now.Add(time.Second))
}
