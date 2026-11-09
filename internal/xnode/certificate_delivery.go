package xnode

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

var errCertificateEvidence = errors.New("certificate outcome requires original-agent reconciliation")

// Private queue metadata is not part of the browser command request/fingerprint.
// A replacement agent must never repeat an uncertain external issuance.
type queuedNodeCommand struct {
	CommandRequest
	AgentID string `json:"_agentId,omitempty"`
}

func (s *Service) sealCommandIntent(actor, key string, node Node, request CommandRequest, raw []byte) (string, error) {
	if request.Name == wire.CertificateRunCommand {
		if node.AgentID == "" {
			return "", ErrUnavailable
		}
		var err error
		raw, err = json.Marshal(queuedNodeCommand{CommandRequest: request, AgentID: node.AgentID})
		if err != nil {
			return "", err
		}
	}
	return s.vault.Encrypt("xnode.command."+actor+"."+key, string(raw))
}

func (s *Service) operationArguments(ctx context.Context, nodeID string, operation providerops.Operation, intent queuedNodeCommand) (map[string]any, error) {
	request := intent.CommandRequest
	if request.Name != wire.CertificateRunCommand {
		return s.commandArguments(ctx, nodeID, request)
	}
	node, err := s.repo.XNodeNode(ctx, nodeID)
	if err != nil {
		if operation.Attempts > 0 && isMissing(err) {
			return nil, errCertificateEvidence
		}
		return nil, err
	}
	if intent.AgentID == "" || node.AgentID != intent.AgentID {
		if operation.Attempts > 0 || intent.AgentID == "" {
			return nil, errCertificateEvidence
		}
		return nil, ErrConflict
	}
	if s.master == nil {
		return nil, ErrUnavailable
	}
	s.master.mu.RLock()
	peer := s.master.peers[nodeID]
	peerMatches := peer != nil && peer.agentID == intent.AgentID
	s.master.mu.RUnlock()
	if peer == nil {
		return nil, ErrUnavailable
	}
	if !peerMatches {
		return nil, errCertificateEvidence
	}
	if operation.Attempts == 0 {
		return s.certificateArguments(ctx, nodeID, request, true)
	}
	if !containsCapability(s.master.Capabilities(nodeID), wire.CertificateLifecycleCapability) {
		return nil, errCertificateEvidence
	}
	// The original durable job ID is replayed before agent eligibility checks.
	// Current profile/revision changes cannot turn replay into a fresh issuance.
	return map[string]any{"profile_id": request.ProfileID, "action": request.Action, "expected_revision": request.ExpectedRevision}, nil
}

func (s *Service) completeCertificateReview(ctx context.Context, repository operationRepository, operation providerops.Operation, node string, intent queuedNodeCommand) error {
	facts, err := certificateOperationResult(intent.CommandRequest, node, nil, intent.AgentID)
	if err != nil {
		return err
	}
	finish, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_, err = repository.CompleteXNodeProviderOperation(finish, operation.Receipt.ID, operation.Attempts,
		providerops.Completion{Status: providerops.StatusPendingReview, ErrorCode: "XNODE_CERTIFICATE_UNCONFIRMED", ResultJSON: facts}, s.now().UTC())
	return err
}

func (m *Master) certificateCommand(ctx context.Context, node, id string, args map[string]any, agent string) (json.RawMessage, error) {
	m.mu.RLock()
	peer := m.peers[node]
	compatible := peer != nil && peer.agentID == agent && containsCapability(m.live[node].hello.Capabilities, wire.CertificateLifecycleCapability)
	m.mu.RUnlock()
	if peer == nil {
		return nil, &CommandError{CommandUndelivered, ErrUnavailable}
	}
	if !compatible {
		return nil, &CommandError{CommandUncertain, errCertificateEvidence}
	}
	// Pin the exact authenticated peer. Replacement cancels that peer's context;
	// re-resolving by node ID here could deliver through a newly enrolled agent.
	return m.commandPeer(ctx, peer, id, wire.CertificateRunCommand, args)
}
