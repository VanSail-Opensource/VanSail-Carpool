package xnode

import (
	"context"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"time"
)

// SubscriptionReadiness survives outages after owner-specific native proof.
type SubscriptionReadiness struct {
	Ready         bool       `json:"ready"`
	FirstSyncedAt *time.Time `json:"firstSyncedAt"`
}

func (s *Service) SubscriptionReadiness(ctx context.Context, identity Identity) (SubscriptionReadiness, error) {
	result := SubscriptionReadiness{Ready: identity.UserID == "" || identity.FirstSyncedAt != nil, FirstSyncedAt: identity.FirstSyncedAt}
	if result.Ready || s.master == nil {
		return result, nil
	}
	// No historical node-wide revision can prove which principals were applied.
	// Ask the existing per-node reconciler to send a fresh authenticated snapshot.
	proxies, err := s.MemberProxies(ctx, identity.UserID)
	if err != nil {
		return result, err
	}
	s.master.mu.RLock()
	defer s.master.mu.RUnlock()
	nodes := map[string]bool{}
	for _, proxy := range proxies {
		nodes[proxy.NodeID] = true
	}
	for node := range nodes {
		if peer := s.master.peers[node]; peer != nil {
			s.master.requestSync(peer)
		}
	}
	return result, nil
}

type appliedMemberConfig struct {
	revision   uint64
	principals []FirstSyncSubject
	validUntil map[int64]time.Time
	appliedAt  time.Time
}

func firstSyncCandidates(config wire.Config, revision uint64, now time.Time, subjects map[int64]FirstSyncSubject) *appliedMemberConfig {
	evidence := &appliedMemberConfig{revision: revision, validUntil: map[int64]time.Time{}}
	for _, user := range config.Users {
		if user.ID > 0 && user.AccountingPeriod != "" && !user.BlockNew && len(user.InboundTags) > 0 && len(user.Credentials) > 0 && (user.ValidUntil == nil || user.ValidUntil.After(now)) {
			subject, ok := subjects[int64(user.ID)]
			if !ok || subject.ProxyUserID != int64(user.ID) || subject.PeriodID != user.AccountingPeriod {
				continue
			}
			evidence.principals = append(evidence.principals, subject)
			if user.ValidUntil != nil {
				evidence.validUntil[int64(user.ID)] = *user.ValidUntil
			}
		}
	}
	return evidence
}

// recordFirstSyncAck only stages immutable configuration membership. It performs
// no SQL or agent command in the socket reader and fences replaced peers.
func (m *Master) recordFirstSyncAck(peer *agentPeer, revision uint64) {
	m.mu.RLock()
	if m.peers[peer.nodeID] != peer {
		m.mu.RUnlock()
		return
	}
	peer.mu.Lock()
	if evidence := peer.firstSync; evidence != nil && evidence.revision == revision && peer.revision == revision && evidence.appliedAt.IsZero() {
		evidence.appliedAt = m.service.Now()
		eligible := make([]FirstSyncSubject, 0, len(evidence.principals))
		for _, subject := range evidence.principals {
			if deadline := evidence.validUntil[subject.ProxyUserID]; deadline.IsZero() || deadline.After(evidence.appliedAt) {
				eligible = append(eligible, subject)
			}
		}
		evidence.principals = eligible
	}
	peer.mu.Unlock()
	m.mu.RUnlock()
	select {
	case m.stateWake <- struct{}{}:
	default:
	}
}

// Called by existing joined reconciliation workers, independently of traffic ACKs.
func (m *Master) confirmFirstSync(ctx context.Context, peer *agentPeer) error {
	repo, ok := m.service.repo.(interface {
		ConfirmXNodeFirstSync(context.Context, string, uint64, []FirstSyncSubject, time.Time) error
	})
	if !ok {
		return nil
	}
	peer.mu.Lock()
	evidence := peer.firstSync
	if evidence == nil || evidence.appliedAt.IsZero() {
		peer.mu.Unlock()
		return nil
	}
	copy := *evidence
	peer.mu.Unlock()
	for start := 0; start < len(copy.principals); start += 256 {
		if err := repo.ConfirmXNodeFirstSync(ctx, peer.nodeID, copy.revision, copy.principals[start:min(start+256, len(copy.principals))], copy.appliedAt); err != nil {
			return err
		}
	}
	peer.mu.Lock()
	if peer.firstSync == evidence {
		peer.firstSync = nil
	}
	peer.mu.Unlock()
	return nil
}
