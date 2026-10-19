package xnode

import (
	"context"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// Country prefix generations travel inside the ordinary state snapshot. The
// pinned agent contract has no separate streaming command, so deliveries wake
// the existing reconciler and reuse the cached compiled document.
func (m *Master) deliverCountry(ctx context.Context, node string) error {
	config, err := m.service.Compile(ctx, node)
	if err != nil {
		return err
	}
	return m.ensureCountry(ctx, node, config.Firewall.SourceGeoIP)
}

func (m *Master) ensureCountry(ctx context.Context, node string, policy *wire.SourceGeoIPPolicy) error {
	if policy == nil {
		return nil
	}
	m.mu.RLock()
	peer := m.peers[node]
	m.mu.RUnlock()
	if peer == nil || peer.ctx.Err() != nil {
		return ErrUnavailable
	}
	m.requestSync(peer)
	return nil
}
