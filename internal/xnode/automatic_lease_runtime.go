package xnode

import (
	"cmp"
	"context"
	"slices"
	"sync"
	"time"
)

func (m *Master) runAutomaticLeases() {
	defer m.wg.Done()
	timer := time.NewTicker(time.Second)
	defer timer.Stop()
	var workers sync.WaitGroup
	defer workers.Wait()
	slots := make(chan struct{}, 2)
	cursor := 0
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-timer.C:
		case <-m.automaticWake:
		}
		if m.ctx.Err() != nil {
			return
		}
		m.automaticLeases.expire(m.service.Now())
		release, cancel := context.WithTimeout(m.ctx, 5*time.Second)
		for range 16 {
			worked, err := m.releaseAutomaticLeaseBans(release)
			if err != nil || !worked {
				break
			}
		}
		cancel()
		generation, enabled := m.automaticLeases.currentGeneration()
		if !enabled {
			continue
		}
		m.mu.RLock()
		peers := make([]*agentPeer, 0, len(m.peers))
		for _, peer := range m.peers {
			peers = append(peers, peer)
		}
		m.mu.RUnlock()
		slices.SortFunc(peers, func(a, b *agentPeer) int { return cmp.Compare(a.nodeID, b.nodeID) })
		start := cursor
		for offset := range len(peers) {
			index := (start + offset) % len(peers)
			peer := peers[index]
			if !m.automaticLeaseEligible(peer) {
				continue
			}
			peer.mu.Lock()
			if peer.automaticRunning || peer.automaticCursor >= generation || peer.automaticRetryAt.After(m.service.Now()) {
				peer.mu.Unlock()
				continue
			}
			selected := false
			select {
			case slots <- struct{}{}:
				selected = true
			default:
			}
			if !selected {
				peer.mu.Unlock()
				break
			}
			peer.automaticRunning = true
			peer.mu.Unlock()
			cursor = (index + 1) % len(peers)
			workers.Go(func() {
				defer func() { <-slots }()
				call, stop := context.WithTimeout(m.ctx, 5*time.Second)
				more, err := m.deliverAutomaticLeases(call, peer)
				stop()
				peer.mu.Lock()
				peer.automaticRunning = false
				if err != nil {
					peer.automaticRetryDelay = min(30*time.Second, max(time.Second, peer.automaticRetryDelay*2))
					peer.automaticRetryAt = m.service.Now().Add(peer.automaticRetryDelay)
				}
				peer.mu.Unlock()
				if err == nil && more {
					m.notifyAutomaticLeases()
				}
			})
		}
	}
}

func (m *automaticLeaseManager) currentGeneration() (uint64, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.generation, m.config.Enabled
}
