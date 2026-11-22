package xnode

import (
	"maps"
	"slices"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type publishedPAYGLease struct {
	period string
	until  time.Time
}

// Caller holds syncMu. A stale projection can avoid a redundant send but must
// never enter an outbound frame (including a revisioned user.controls update).
func (m *Master) prepareAuthorizationWire(peer *agentPeer, config wire.Config, capabilities []string, force bool) (*preparedWire, error) {
	prepared, err := preparePeerWire(peer, config, capabilities)
	if err != nil || !peer.paygHeld {
		return prepared, err
	}
	controls := m.connectionControls(prepared.config)
	peer.mu.Lock()
	unchanged := !force && peer.digest == prepared.digest && maps.Equal(controls, peer.lastControls)
	peer.mu.Unlock()
	if unchanged {
		return prepared, nil
	}
	config.Users = slices.DeleteFunc(slices.Clone(config.Users), func(user wire.User) bool {
		return isPAYGPeriod(user.AccountingPeriod)
	})
	peer.paygHeld = false
	peer.wireState = nil
	peer.paygOverlayKey = ""
	return preparePeerWire(peer, config, capabilities)
}

func (peer *agentPeer) rememberPublishedPAYG(users []wire.User) {
	peer.paygPublished = nil
	var until time.Time
	for _, user := range users {
		if user.ValidUntil == nil || !isPAYGPeriod(user.AccountingPeriod) {
			continue
		}
		if peer.paygPublished == nil {
			peer.paygPublished = make(map[int]publishedPAYGLease)
		}
		peer.paygPublished[user.ID] = publishedPAYGLease{period: user.AccountingPeriod, until: *user.ValidUntil}
		if until.IsZero() || user.ValidUntil.Before(until) {
			until = *user.ValidUntil
		}
	}
	peer.mu.Lock()
	peer.paygPublishedUntil = until
	peer.mu.Unlock()
}
