package xnode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

const DDNSOperationKind = "xnode_ddns_update"

// DDNSChangeState is durable desired mutation/order evidence, not telemetry.
// Prior addresses remain protected until endpoint and route references converge.
type DDNSChangeState struct {
	NodeID, AgentID, OperationID, Phase string
	Change                              wire.DDNSChange
	PreviousIPs                         []string
}

type DDNSChangeInput struct {
	Node        Node
	Version     int64
	Binding     DDNSHostBinding
	Change      wire.DDNSChange
	Operation   providerops.CreateInput
	PreviousIPs []string
}

type ddnsChangeRepository interface {
	AcceptXNodeDDNSChange(context.Context, DDNSChangeInput, time.Time) (bool, error)
	XNodeDDNSChanges(context.Context) ([]DDNSChangeState, error)
}

func (s *Service) acceptDDNSChange(ctx context.Context, node, agent string, change wire.DDNSChange) error {
	in, err := s.prepareDDNSChange(ctx, node, agent, change)
	if err != nil || in == nil {
		return err
	}
	repo, ok := s.repo.(ddnsChangeRepository)
	if !ok {
		return ErrUnavailable
	}
	_, err = repo.AcceptXNodeDDNSChange(ctx, *in, s.now().UTC())
	return err
}

func (s *Service) prepareDDNSChange(ctx context.Context, node, agent string, change wire.DDNSChange) (*DDNSChangeInput, error) {
	if change.Validate() != nil {
		return nil, ErrInvalid
	}
	versions, versioned := s.repo.(compileVersionRepository)
	if !versioned {
		return nil, ErrUnavailable
	}
	version, _, err := versions.XNodeCompileVersion(ctx, s.now())
	if err != nil {
		return nil, err
	}
	current, settings, err := s.Node(ctx, node)
	if err != nil {
		return nil, err
	}
	if current.AgentID != agent {
		return nil, ErrConflict
	}
	global, err := s.Global(ctx)
	if err != nil {
		return nil, err
	}
	for _, job := range ddnsJobsForNode(node, settings, global.Providers) {
		if job.ID != change.JobID {
			continue
		}
		// A queued event from a replaced configuration is obsolete. Acknowledge
		// its disposal so it cannot fill the agent's durable replay journal.
		if job.Generation == "" || job.Generation != change.Generation {
			return nil, nil
		}
		if !ddnsAddressMatches(job.IPv4, change.IPv4) || !ddnsAddressMatches(job.IPv6, change.IPv6) {
			return nil, ErrInvalid
		}
		raw, err := json.Marshal([]any{node, agent, change})
		if err != nil {
			return nil, err
		}
		digest := sha256.Sum256(raw)
		key := sha256.Sum256([]byte(node + "\n" + agent + "\n" + change.ID))
		binding := settings.DDNSBindings[job.ID]
		input := DDNSChangeInput{Node: current, Version: version, Binding: binding, Change: change, PreviousIPs: s.previousDDNSOrigin(node, job), Operation: providerops.CreateInput{
			ActorUserID: binding.ConfiguredBy, Kind: DDNSOperationKind,
			IdempotencyKey: "ddns-" + hex.EncodeToString(key[:]), RequestFingerprint: hex.EncodeToString(digest[:]),
			Items: []providerops.ItemInput{{Key: "ddns", TargetType: "xnode_ddns", TargetID: node}},
		}}
		return &input, nil
	}
	return nil, nil // deleted jobs are obsolete too
}

func ddnsAddressMatches(source, address string) bool {
	if source == "" {
		return address == ""
	}
	return address != "" && (source == "auto" || source == address)
}

func (m *Master) receiveDDNSChanges(ctx context.Context, peer *agentPeer, message wire.Message) error {
	if !peer.ddnsChanges || len(message.Data) > 32<<10 {
		return ErrInvalid
	}
	m.mu.RLock()
	current := m.peers[peer.nodeID] == peer
	m.mu.RUnlock()
	if !current {
		return ErrConflict
	}
	var report wire.DDNSChanges
	if wire.Decode(message.Data, &report) != nil || len(report.Changes) == 0 || len(report.Changes) > wire.MaxDDNSChangesPerFrame {
		return ErrInvalid
	}
	ack := wire.DDNSChangeAck{IDs: []string{}}
	for _, change := range report.Changes {
		if err := m.service.acceptDDNSChange(ctx, peer.nodeID, peer.agentID, change); err != nil {
			return err
		}
		ack.IDs = append(ack.IDs, change.ID)
	}
	// The same single writer sends acknowledgments only after durable acceptance
	// or validated disposal. Partial commits replay safely if a later item fails.
	return peer.send(ctx, wire.New("ddns.changes.ack", message.ID, 0, ack))
}
