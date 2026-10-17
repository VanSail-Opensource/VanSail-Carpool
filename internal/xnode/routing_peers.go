package xnode

import (
	"context"
	"sync"
	"time"
)

type routingPeer struct {
	address string
	until   time.Time
}
type routingRuntime struct {
	legacyBefore time.Time
	mu           sync.Mutex
	peers        map[string]routingPeer
	revision     int64
	local        int64
	checked      bool
	next         time.Time
}

func (s *Service) resolveRoutingPeer(ctx context.Context, address string) (string, error) {
	r := s.routingRuntime
	r.mu.Lock()
	defer r.mu.Unlock()
	if peer, ok := r.peers[address]; ok && s.now().Before(peer.until) {
		return peer.address, nil
	}
	ip, err := s.resolvePublicProxy(ctx, address)
	if err != nil {
		return "", err
	}
	if len(r.peers) >= 4096 {
		oldest := ""
		var until time.Time
		for key, p := range r.peers {
			if until.IsZero() || p.until.Before(until) {
				oldest, until = key, p.until
			}
		}
		delete(r.peers, oldest)
	}
	r.peers[address] = routingPeer{address: ip, until: s.now().Add(5 * time.Minute)}
	return ip, nil
}
func (s *Service) routingPeerRefreshAt() time.Time {
	r := s.routingRuntime
	r.mu.Lock()
	defer r.mu.Unlock()
	var next time.Time
	for _, peer := range r.peers {
		if s.now().Before(peer.until) && (next.IsZero() || peer.until.Before(next)) {
			next = peer.until
		}
	}
	return next
}
func (s *Service) routingReconcileNeeded(ctx context.Context) (func(bool), error) {
	repo, ok := s.repo.(compileVersionRepository)
	if !ok {
		return func(bool) {}, nil
	}
	version, inputDeadline, err := repo.XNodeCompileVersion(ctx, s.now())
	if err != nil {
		return nil, err
	}
	local := s.localCompileRevision.Load()
	r := s.routingRuntime
	r.mu.Lock()
	skip := r.checked && r.revision == version && r.local == local && (r.next.IsZero() || s.now().Before(r.next))
	r.mu.Unlock()
	if skip {
		return nil, nil
	}
	return func(success bool) {
		if !success {
			return
		}
		next := s.routingRefreshAt()
		peer := s.routingPeerRefreshAt()
		if !peer.IsZero() && (next.IsZero() || peer.Before(next)) {
			next = peer
		}
		if !inputDeadline.IsZero() && (next.IsZero() || inputDeadline.Before(next)) {
			next = inputDeadline
		}
		r.mu.Lock()
		r.checked, r.revision, r.local, r.next = true, version, local, next
		r.mu.Unlock()
	}, nil
}
