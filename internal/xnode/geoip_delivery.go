package xnode

import (
	"context"
	"encoding/json"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type countryQueueRepository interface {
	QueueXNodeGeoIP(context.Context, string) error
}

func (m *Master) countryReady(peer *agentPeer, generation string) bool {
	peer.mu.Lock()
	defer peer.mu.Unlock()
	return peer.geoIPGeneration == generation
}

func (m *Master) queueCountry(ctx context.Context, peer *agentPeer, policy *wire.SourceGeoIPPolicy) error {
	if policy == nil || m.countryReady(peer, policy.Generation) {
		return nil
	}
	repo, ok := m.service.repo.(countryQueueRepository)
	if !ok {
		return ErrUnavailable
	}
	if err := repo.QueueXNodeGeoIP(ctx, peer.nodeID); err != nil {
		return err
	}
	return ErrUnavailable
}

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
	if peer == nil {
		return ErrUnavailable
	}
	if !containsCapability(m.Capabilities(node), "source_geoip_stream_v1") {
		return nil
	}
	peer.countryMu.Lock()
	defer peer.countryMu.Unlock()
	if m.countryReady(peer, policy.Generation) {
		return nil
	}
	probe, err := m.Command(ctx, node, "", "firewall.geoip.sync", wire.GeoIPChunk{Generation: policy.Generation, Probe: true})
	if err != nil {
		return err
	}
	var status struct {
		Ready bool `json:"ready"`
	}
	if json.Unmarshal(probe, &status) != nil {
		return ErrInvalid
	}
	if !status.Ready {
		for offset, index := 0, 0; offset < len(policy.Prefixes) || offset == 0; offset, index = offset+256, index+1 {
			end := min(offset+256, len(policy.Prefixes))
			chunk := wire.GeoIPChunk{Generation: policy.Generation, Index: index, Final: end == len(policy.Prefixes), Prefixes: policy.Prefixes[offset:end]}
			raw, err := m.Command(ctx, node, "", "firewall.geoip.sync", chunk)
			if err != nil {
				return err
			}
			if chunk.Final && (json.Unmarshal(raw, &status) != nil || !status.Ready) {
				return ErrInvalid
			}
			if peer.ctx.Err() != nil {
				return ErrUnavailable
			}
			if chunk.Final {
				break
			}
		}
	}
	peer.mu.Lock()
	peer.geoIPGeneration = policy.Generation
	peer.mu.Unlock()
	m.requestSync(peer)
	return nil
}

func referenceCountry(config wire.Config) wire.Config {
	if config.Firewall.SourceGeoIP == nil {
		return config
	}
	copy := *config.Firewall.SourceGeoIP
	copy.Prefixes = nil
	copy.Reference = true
	config.Firewall.SourceGeoIP = &copy
	return config
}
