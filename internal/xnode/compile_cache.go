package xnode

import (
	"context"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type compileVersionRepository interface {
	XNodeCompileVersion(context.Context, time.Time) (int64, time.Time, error)
}

type compiledState struct {
	version      int64
	localVersion int64
	until        time.Time
	config       wire.Config
}

// cachedConfiguration never persists telemetry or admission decisions. The
// repository epoch changes in the same transaction as authoritative inputs.
func (m *Master) cachedConfiguration(ctx context.Context, peer *agentPeer, force bool) (wire.Config, error) {
	repo, supported := m.service.repo.(compileVersionRepository)
	if !supported {
		return m.service.Compile(ctx, peer.nodeID)
	}
	now := m.service.now()
	localVersion := m.service.localCompileRevision.Load()
	version, next, err := repo.XNodeCompileVersion(ctx, now)
	if err != nil {
		return wire.Config{}, err
	}
	if saved := peer.compiled; !force && saved != nil && saved.version == version && saved.localVersion == localVersion && (saved.until.IsZero() || now.Before(saved.until)) {
		return saved.config, nil
	}
	config, err := m.service.Compile(ctx, peer.nodeID)
	if err != nil {
		return config, err
	}
	for _, user := range config.Users {
		if user.ValidUntil != nil && user.ValidUntil.After(now) && (next.IsZero() || user.ValidUntil.Before(next)) {
			next = *user.ValidUntil
		}
	}
	// Never publish a candidate compiled across a concurrently committed change.
	after, _, err := repo.XNodeCompileVersion(ctx, now)
	if err != nil {
		return wire.Config{}, err
	}
	if after != version || m.service.localCompileRevision.Load() != localVersion {
		return wire.Config{}, ErrConflict
	}
	peer.compiled = &compiledState{version: version, localVersion: localVersion, until: next, config: config}
	return config, nil
}

func (m *Master) connectionControls(config wire.Config) map[int]bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	controls := make(map[int]bool, len(config.Users))
	for _, user := range config.Users {
		controls[user.ID] = m.globalBlocks[user.ID]
	}
	return controls
}
