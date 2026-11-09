package xnode

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type certificateOperationRepository interface {
	CreateXNodeCertificateOperation(context.Context, providerops.CreateInput, string, string, uint64, time.Time) (providerops.Operation, bool, error)
	LatestXNodeCertificateOperation(context.Context, string, string) (providerops.Operation, error)
}

// CertificateOperation is the existing durable receipt plus safe intent/results.
type CertificateOperation struct {
	Receipt *model.OperationReceipt `json:"receipt"`
	Result  json.RawMessage         `json:"result"`
}

func validCertificateRequest(request CommandRequest) bool {
	return wire.ValidName(request.ProfileID) && (request.Action == "issue" || request.Action == "renew") &&
		request.ExpectedRevision > 0 && request.ExpectedRevision <= 1<<53-1 &&
		request.JobID == "" && request.Version == "" && request.PeerEndpointID == "" && !request.Force
}

func (s *Service) certificateArguments(ctx context.Context, nodeID string, request CommandRequest, requirePeer bool) (map[string]any, error) {
	if !validCertificateRequest(request) {
		return nil, ErrInvalid
	}
	node, settings, err := s.Node(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	if !node.Enabled || node.RemovingAt != nil || node.RetiredAt != nil || node.DesiredRevision != request.ExpectedRevision {
		return nil, ErrConflict
	}
	profile, exists := settings.Certificates[request.ProfileID]
	mode := certificateMode(profile.Config)
	if !exists || mode != "http" && mode != "dns" {
		return nil, ErrInvalid
	}
	connected := s.master != nil && s.master.Connected(nodeID)
	if requirePeer && !connected {
		return nil, ErrUnavailable
	}
	if connected && !containsCapability(s.master.Capabilities(nodeID), wire.CertificateLifecycleCapability) {
		return nil, &CapabilityError{Capability: wire.CertificateLifecycleCapability}
	}
	return map[string]any{"profile_id": request.ProfileID, "action": request.Action, "expected_revision": request.ExpectedRevision}, nil
}

// LatestCertificateOperation supports reopening a modal without another issuance.
func (s *Service) LatestCertificateOperation(ctx context.Context, node, profile string) (CertificateOperation, error) {
	result := CertificateOperation{Result: json.RawMessage(`{}`)}
	if node == "" || !wire.ValidName(profile) {
		return result, ErrInvalid
	}
	repository, ok := s.repo.(certificateOperationRepository)
	if !ok {
		return result, ErrUnavailable
	}
	operation, err := repository.LatestXNodeCertificateOperation(ctx, node, profile)
	if isMissing(err) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	result.Receipt = &operation.Receipt
	if operation.ResultJSON != "" {
		result.Result = json.RawMessage(operation.ResultJSON)
	}
	return result, nil
}

func validateCertificateResult(request CommandRequest, raw json.RawMessage) (json.RawMessage, error) {
	var result wire.CertificateRunResult
	if json.Unmarshal(raw, &result) != nil || result.ProfileID != request.ProfileID || result.Action != request.Action ||
		!result.Applied || !result.Ready || !certificateFingerprint(result.Fingerprint) {
		return raw, &CommandError{CommandUncertain, ErrInvalid}
	}
	return raw, nil
}

func certificateOperationResult(request CommandRequest, node string, raw json.RawMessage, agent ...string) (string, error) {
	facts := map[string]any{"command": request.Name, "nodeId": node, "profile_id": request.ProfileID, "action": request.Action, "expected_revision": request.ExpectedRevision}
	if len(agent) > 0 && agent[0] != "" {
		facts["agent_id"] = agent[0]
	}
	if len(raw) > 0 {
		var result wire.CertificateRunResult
		if json.Unmarshal(raw, &result) != nil || result.ProfileID != request.ProfileID || result.Action != request.Action {
			// Bad/uncertain agent evidence cannot erase the durable original intent.
			encoded, err := json.Marshal(facts)
			return string(encoded), err
		}
		facts["applied"], facts["ready"] = result.Applied, result.Ready
		if certificateFingerprint(result.Fingerprint) {
			facts["fingerprint"] = result.Fingerprint
		}
		if !result.Expires.IsZero() {
			facts["expires"] = result.Expires
		}
	}
	encoded, err := json.Marshal(facts)
	return string(encoded), err
}

func certificateFingerprint(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
