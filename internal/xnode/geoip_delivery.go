package xnode

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

const geoIPQueueRetryInterval = 30 * time.Second

type geoIPQueueRepository interface {
	QueueXNodeGeoIP(context.Context, string) error
}

// deliverCountry runs from the durable outbox. It stages the current resolved
// generation before waking the ordinary state reconciler to publish its ref.
func (m *Master) deliverCountry(ctx context.Context, node string) error {
	config, err := m.service.Compile(ctx, node)
	if err != nil {
		return err
	}
	policy := config.Firewall.SourceGeoIP
	if policy == nil {
		return nil
	}
	m.mu.RLock()
	peer := m.peers[node]
	m.mu.RUnlock()
	if peer == nil || peer.ctx.Err() != nil {
		return ErrUnavailable
	}
	if !containsCapability(m.Capabilities(node), "source_geoip_stream_v1") {
		m.requestSync(peer)
		return nil
	}
	err = m.ensureCountry(ctx, peer, policy)
	if err != nil {
		return err
	}
	m.requestSync(peer)
	return nil
}

// prepareCountryForPeer queues country staging before state activation.
// Older agents keep the bounded inline form from the original contract.
func (m *Master) prepareCountryForPeer(ctx context.Context, peer *agentPeer, config wire.Config, capabilities []string) (wire.Config, bool, error) {
	policy := config.Firewall.SourceGeoIP
	if policy == nil {
		peer.geoIPStateMu.Lock()
		peer.geoIPGeneration = ""
		peer.geoIPQueuedGeneration = ""
		peer.geoIPQueuedAt = time.Time{}
		peer.geoIPStateMu.Unlock()
		return config, true, nil
	}
	copy := *policy
	config.Firewall.SourceGeoIP = &copy
	if !containsCapability(capabilities, "source_geoip_stream_v1") {
		copy.Reference = false
		return config, true, nil
	}
	peer.geoIPStateMu.Lock()
	ready := peer.geoIPGeneration == copy.Generation
	queue := !ready && (peer.geoIPQueuedGeneration != copy.Generation || peer.geoIPQueuedAt.IsZero() || m.service.now().Sub(peer.geoIPQueuedAt) >= geoIPQueueRetryInterval)
	peer.geoIPStateMu.Unlock()
	if !ready {
		now := m.service.now()
		if queue {
			repo, ok := m.service.repo.(geoIPQueueRepository)
			if !ok {
				return config, false, ErrUnavailable
			}
			if err := repo.QueueXNodeGeoIP(ctx, peer.nodeID); err != nil {
				return config, false, err
			}
			peer.geoIPStateMu.Lock()
			peer.geoIPQueuedGeneration = copy.Generation
			peer.geoIPQueuedAt = now
			peer.geoIPStateMu.Unlock()
		}
		return config, false, nil
	}
	copy.Reference = true
	copy.Prefixes = nil
	return config, true, nil
}

func (m *Master) ensureCountry(ctx context.Context, peer *agentPeer, policy *wire.SourceGeoIPPolicy) error {
	if policy == nil {
		return nil
	}
	if !peer.geoIPTransferMu.TryLock() {
		return outbox.ErrBusy
	}
	defer peer.geoIPTransferMu.Unlock()
	peer.geoIPStateMu.Lock()
	readyGeneration := peer.geoIPGeneration == policy.Generation
	peer.geoIPStateMu.Unlock()
	if readyGeneration {
		return nil
	}
	ready, err := m.countryGenerationReady(ctx, peer.nodeID, wire.GeoIPChunk{Generation: policy.Generation, Probe: true})
	if err != nil {
		return err
	}
	if !ready {
		if len(policy.Prefixes) == 0 {
			return ErrInvalid
		}
		for index := 0; ; index++ {
			chunk, ok := countryChunk(policy, index)
			if !ok {
				break
			}
			ready, err = m.countryGenerationReady(ctx, peer.nodeID, chunk)
			if err != nil {
				return err
			}
			if chunk.Final && !ready {
				return ErrUnavailable
			}
		}
	}
	peer.geoIPStateMu.Lock()
	peer.geoIPGeneration = policy.Generation
	peer.geoIPQueuedGeneration = policy.Generation
	peer.geoIPQueuedAt = m.service.now()
	peer.geoIPStateMu.Unlock()
	return nil
}

func countryChunk(policy *wire.SourceGeoIPPolicy, index int) (wire.GeoIPChunk, bool) {
	start := index * wire.GeoIPPrefixesPerChunk
	if policy == nil || index < 0 || start >= len(policy.Prefixes) {
		return wire.GeoIPChunk{}, false
	}
	end := min(start+wire.GeoIPPrefixesPerChunk, len(policy.Prefixes))
	return wire.GeoIPChunk{Generation: policy.Generation, Index: index, Final: end == len(policy.Prefixes), Prefixes: policy.Prefixes[start:end]}, true
}

func (m *Master) prepareCountryReference(ctx context.Context, node string, config wire.Config, capabilities []string) (wire.Config, error) {
	policy := config.Firewall.SourceGeoIP
	if policy == nil {
		return config, nil
	}
	if !containsCapability(capabilities, "source_geoip_stream_v1") {
		raw, err := json.Marshal(config)
		if err != nil {
			return config, ErrInvalid
		}
		if len(raw) > wire.MaxFrameBytes-1024 {
			return config, &CapabilityError{Capability: "source_geoip_stream_v1"}
		}
		return config, nil
	}
	m.mu.RLock()
	peer := m.peers[node]
	m.mu.RUnlock()
	if peer == nil || peer.ctx.Err() != nil {
		return config, ErrUnavailable
	}
	err := m.ensureCountry(ctx, peer, policy)
	if err != nil {
		if errors.Is(err, outbox.ErrBusy) {
			return config, geoIPPolicyFailure(ErrUnavailable)
		}
		return config, err
	}
	copy := *policy
	copy.Reference = true
	copy.Prefixes = nil
	config.Firewall.SourceGeoIP = &copy
	return config, nil
}

func (m *Master) countryGenerationReady(ctx context.Context, node string, chunk wire.GeoIPChunk) (bool, error) {
	raw, err := m.Command(ctx, node, "", "firewall.geoip.sync", chunk)
	if err != nil {
		return false, err
	}
	var response struct {
		Ready bool `json:"ready"`
	}
	if wire.Decode(raw, &response) != nil {
		return false, ErrInvalid
	}
	return response.Ready, nil
}
