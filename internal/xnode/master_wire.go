package xnode

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type preparedWire struct {
	source *compiledState
	config wire.Config
	digest string
	err    error
}

// Called under syncMu. The candidate and negotiated hello capabilities are
// immutable for this peer. Controls are projected separately on each interval.
func preparePeerWire(peer *agentPeer, config wire.Config, capabilities []string) (*preparedWire, error) {
	if saved := peer.wireState; peer.compiled != nil && saved != nil && saved.source == peer.compiled {
		return saved, saved.err
	}
	p := &preparedWire{source: peer.compiled, config: firewallForCapabilities(config, capabilities)}
	peer.wireState = p
	for _, user := range config.Users {
		if user.ReplacesID != 0 && !containsCapability(capabilities, "telegram_identity_rekey_v1") {
			p.err = &CapabilityError{Capability: "telegram_identity_rekey_v1"}
			return p, p.err
		}
	}
	if p.err = configurationCapabilities(p.config, capabilities); p.err != nil {
		return p, p.err
	}
	// Copy just the slice whose scalar controls change. The validated document,
	// CIDRs and credential maps stay shared and immutable, avoiding JSON cloning.
	p.config.Users = slices.Clone(config.Users)
	for i := range p.config.Users {
		p.config.Users[i].BlockNew = false
	}
	raw, err := json.Marshal(p.config)
	if err != nil {
		p.err = ErrInvalid
		return p, p.err
	}
	if len(raw) > wire.MaxFrameBytes-1024 {
		if policy := p.config.Firewall.SourceGeoIP; policy != nil && !policy.Reference {
			p.err = &CapabilityError{Capability: "source_geoip_stream_v1"}
		} else {
			p.err = ErrInvalid
		}
		return p, p.err
	}
	hash := sha256.Sum256(raw)
	p.digest = hex.EncodeToString(hash[:])
	return p, nil
}
