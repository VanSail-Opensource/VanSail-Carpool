package xnode

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func (s *Service) executeNodeCommand(ctx context.Context, node string, operation providerops.Operation, request CommandRequest, args map[string]any, agent ...string) (json.RawMessage, error) {
	if request.Name == "speedtest" && operation.Attempts > 1 {
		return nil, &CommandError{CommandUncertain, ErrUnavailable}
	}
	if isUpdateCommand(request.Name) {
		return s.executeUpdate(ctx, node, operation, request, args)
	}
	var result json.RawMessage
	var err error
	if request.Name == "certificates.run" {
		if len(agent) == 0 || agent[0] == "" {
			return nil, &CommandError{CommandUncertain, errCertificateEvidence}
		}
		result, err = s.master.certificateCommand(ctx, node, operation.Receipt.ID, args, agent[0])
	} else {
		result, err = s.master.Command(ctx, node, operation.Receipt.ID, request.Name, args)
	}
	if err == nil && request.Name == "certificates.run" {
		return validateCertificateResult(request, result)
	}
	if err == nil && (request.Name == "runtime.reload" || request.Name == "geodata.refresh") {
		var activation struct {
			Applied bool `json:"applied"`
			Ready   bool `json:"ready"`
		}
		if json.Unmarshal(result, &activation) != nil || !activation.Applied || !activation.Ready {
			return nil, &CommandError{CommandFailed, ErrUnavailable}
		}
	}
	return result, err
}

func (s *Service) executeUpdate(ctx context.Context, node string, operation providerops.Operation, request CommandRequest, args map[string]any) (json.RawMessage, error) {
	expected, ok := UpdateExpectationFromResult(operation.ResultJSON)
	if !ok || expected.Command != request.Name || (request.Name == "update.install" &&
		(expected.TransactionID != operation.Receipt.ID || (request.Version != "" && expected.Version != request.Version))) {
		return nil, &CommandError{CommandUncertain, ErrUpdateEvidence}
	}
	// Attempts and their intent survive restart. An interrupted mutation is never sent again.
	if operation.Attempts > 1 {
		return s.waitUpdate(ctx, node, expected)
	}
	var dispatchPeer *agentPeer
	if request.Name == "update.rollback" {
		transaction, peer, err := s.master.updateTransaction(ctx, node)
		if err != nil {
			return nil, &CommandError{CommandUndelivered, err}
		}
		if !expected.matches(transaction) {
			return nil, &CommandError{CommandUncertain, ErrUpdateEvidence}
		}
		if transaction.Status == "rolled_back" {
			return s.waitUpdate(ctx, node, expected)
		}
		if transaction.Status != "pending" && transaction.Status != "confirmed" {
			return nil, &CommandError{CommandFailed, ErrUpdateEvidence}
		}
		dispatchPeer = peer
	} else {
		s.master.mu.RLock()
		dispatchPeer = s.master.peers[node]
		current := s.master.live[node].hello.Version
		s.master.mu.RUnlock()
		if dispatchPeer == nil {
			return nil, &CommandError{CommandUndelivered, ErrUnavailable}
		}
		if current != expected.Previous && current != expected.Version {
			return nil, &CommandError{CommandUncertain, ErrUpdateEvidence}
		}
	}
	if dispatchPeer == nil {
		return nil, &CommandError{CommandUndelivered, ErrUnavailable}
	}
	result, err := s.master.commandPeer(ctx, dispatchPeer, operation.Receipt.ID, request.Name, args)
	var outcome *CommandError
	if errors.As(err, &outcome) && outcome.Outcome != CommandUncertain {
		return result, err
	}
	if err == nil && request.Name == "update.install" {
		var release struct {
			Available bool   `json:"available"`
			Current   string `json:"current"`
			Version   string `json:"version"`
		}
		if json.Unmarshal(result, &release) == nil && !release.Available && release.Current == expected.Previous &&
			release.Version == expected.Version && (request.Version == "" || release.Current == request.Version) && s.master.updateReadyPeer(node, release.Current, dispatchPeer) {
			// A completed, receipt-bound no-op reply is sufficient. An interrupted no-op
			// still requires recovery evidence and can never use this branch.
			return json.Marshal(map[string]any{"unchanged": true, "current": release.Current})
		}
	}
	return s.waitUpdate(ctx, node, expected)
}
