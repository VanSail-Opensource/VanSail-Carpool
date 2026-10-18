package xnode

import (
	"context"
	"errors"
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

type compileFailure struct {
	version, local int64
	at, boundary   time.Time
	delay          time.Duration
	err            error
}

// cachedConfiguration never persists telemetry or admission decisions. The
// repository epoch changes in the same transaction as authoritative inputs.
func (m *Master) cachedConfiguration(ctx context.Context, peer *agentPeer, _ bool) (wire.Config, error) {
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
	// A full-state resend does not change the authoritative inputs. Reuse the
	// candidate even when the agent requests a baseline after a reconnect gap.
	if saved := peer.compiled; saved != nil && saved.version == version && saved.localVersion == localVersion && (saved.until.IsZero() || now.Before(saved.until)) {
		return saved.config, nil
	}
	failed := peer.compileFailure
	sameFailure := failed != nil && failed.version == version && failed.local == localVersion && (failed.boundary.IsZero() || now.Before(failed.boundary))
	if sameFailure && now.Before(failed.at) {
		return wire.Config{}, failed.err
	}
	config, err := m.service.Compile(ctx, peer.nodeID)
	if err != nil {
		if !errors.Is(err, context.Canceled) && ctx.Err() == nil {
			delay := time.Second
			if sameFailure {
				delay = min(time.Minute, failed.delay*2)
			}
			peer.compileFailure = &compileFailure{version: version, local: localVersion, at: now.Add(delay), boundary: next, delay: delay, err: err}
		}
		return config, err
	}
	for _, user := range config.Users {
		if user.ValidUntil != nil && user.ValidUntil.After(now) && (next.IsZero() || user.ValidUntil.Before(next)) {
			next = *user.ValidUntil
		}
	}
	if config.Firewall.SourceGeoIP != nil {
		if refresh := m.service.policyRefreshAt(); !refresh.IsZero() && (next.IsZero() || refresh.Before(next)) {
			next = refresh
		}
	}
	if refresh := m.service.routingRefreshAt(); !refresh.IsZero() && (next.IsZero() || refresh.Before(next)) {
		next = refresh
	}
	if refresh := m.service.routingPeerRefreshAt(); !refresh.IsZero() && (next.IsZero() || refresh.Before(next)) {
		next = refresh
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
	peer.compileFailure = nil
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
