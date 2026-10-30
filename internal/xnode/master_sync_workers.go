package xnode

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"time"
)

// Two joined consumers share the existing reconciler, not another durable queue.
func (m *Master) syncPeers(ctx context.Context, peers []*agentPeer, now time.Time) error {
	slices.SortFunc(peers, func(a, b *agentPeer) int { return strings.Compare(a.nodeID, b.nodeID) })
	if len(peers) == 0 {
		return nil
	}
	m.mu.Lock()
	start := m.syncCursor % len(peers)
	m.syncCursor = (start + 1) % len(peers)
	m.mu.Unlock()
	jobs := make(chan *agentPeer, len(peers))
	for i := range peers {
		jobs <- peers[(start+i)%len(peers)]
	}
	close(jobs)
	var wg sync.WaitGroup
	for range min(2, len(peers)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for peer := range jobs {
				if ctx.Err() != nil {
					return
				}
				m.syncPeer(ctx, peer, now)
			}
		}()
	}
	wg.Wait()
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
	force := peer.forceSync
	peer.forceSync = false
	peer.mu.Unlock()
	child, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	started := time.Now()
	var err error
	if force {
		if repo, ok := m.service.routingRepository(); ok {
			err = repo.WakeXNodeRouting(child, now)
		}
	}
	if err == nil {
		err = m.syncNodeLocked(child, peer, force)
	}
	if err != nil {
		if force {
			peer.mu.Lock()
			peer.forceSync = true
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
