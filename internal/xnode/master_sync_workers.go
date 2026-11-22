package xnode

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"time"
)

// Preserve a bounded part of the existing pass deadline for durable reconnect
// wakes even when slow peers consume the fleet budget. No work outlives ctx.
func syncPassContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining > 0 {
			reserve := min(time.Second, remaining/5)
			return context.WithDeadline(ctx, deadline.Add(-reserve))
		}
	}
	return context.WithCancel(ctx)
}

// Two joined consumers share the existing reconciler, not another durable queue.
func (m *Master) syncPeers(ctx context.Context, peers []*agentPeer, now time.Time, reportSeconds ...int) error {
	slices.SortFunc(peers, func(a, b *agentPeer) int { return strings.Compare(a.nodeID, b.nodeID) })
	if len(peers) == 0 {
		return nil
	}
	m.mu.Lock()
	start := m.syncCursor % len(peers)
	m.mu.Unlock()
	cadence := 0
	if len(reportSeconds) > 0 {
		cadence = reportSeconds[0]
	}
	jobs := make(chan peerSyncJob, len(peers))
	for _, job := range m.peerSyncOrder(peers, start, now, cadence) {
		jobs <- job
	}
	close(jobs)
	// Each index has one consumer. Read attempts only after both workers join,
	// so queued peers cannot be mistaken for work performed before cancellation.
	attempted := make([]bool, len(peers))
	var wg sync.WaitGroup
	for range min(2, len(peers)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				if ctx.Err() != nil {
					return
				}
				attempted[job.offset] = true
				peer := peers[(start+job.offset)%len(peers)]
				if job.priority {
					peer.mu.Lock()
					peer.paygPriorityAt = now
					peer.mu.Unlock()
				}
				m.syncPeer(ctx, peer, now)
			}
		}()
	}
	wg.Wait()
	// Resume at the first unattempted peer. Advancing by one after a partial
	// fleet pass repeatedly revisits the same slow peers and starves later leases.
	// A complete pass keeps rotating the starting peer, including occupied peers
	// whose TryLock attempt returned immediately.
	next := (start + 1) % len(peers)
	for offset, tried := range attempted {
		if !tried {
			next = (start + offset) % len(peers)
			break
		}
	}
	m.mu.Lock()
	m.syncCursor = next
	m.mu.Unlock()
	return ctx.Err()
}

func (m *Master) syncPeer(ctx context.Context, peer *agentPeer, now time.Time) {
	// A deployment/recovery already publishing this peer must not consume a
	// fleet worker while waiting for the same serialization lock.
	if !peer.syncMu.TryLock() {
		return
	}
	defer peer.syncMu.Unlock()
	peer.mu.Lock()
	force, advance := peer.forceSync, peer.forceAdvance
	peer.forceSync, peer.forceAdvance = false, false
	peer.mu.Unlock()
	child, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	started := time.Now()
	if force {
		// Reconnect evidence must wake durable routing work, but that SQL work
		// cannot precede a short authorization lease. The joined pass coalesces it.
		m.routingWakePending.Store(true)
	}
	err := m.confirmFirstSync(child, peer)
	if err == nil {
		// Queued prerequisites keep the ordinary reconciliation cadence; only
		// explicit Sync callers require proof that publication has happened.
		_, err = m.syncNodeLocked(child, peer, force, !advance)
	}
	if err != nil {
		if force {
			peer.mu.Lock()
			peer.forceSync = true
			peer.forceAdvance = peer.forceAdvance || advance
			peer.mu.Unlock()
		}
		if m.logger != nil {
			stage := "synchronization"
			var failure *synchronizationFailure
			if errors.As(err, &failure) {
				stage = failure.stage
			}
			m.logger.Debug("node synchronization deferred", "node_id", peer.nodeID, "stage", stage, "error_code", safeFrameErrorCode(err), "cause", frameErrorCause(err), "duration_ms", time.Since(started).Milliseconds())
		}
	}
}

type synchronizationFailure struct {
	stage string
	cause error
}

func (e *synchronizationFailure) Error() string { return "X-Node " + e.stage + " pending" }
func (e *synchronizationFailure) Unwrap() error { return e.cause }
func syncFailure(stage string, err error) error {
	if err == nil {
		return nil
	}
	return &synchronizationFailure{stage, err}
}
