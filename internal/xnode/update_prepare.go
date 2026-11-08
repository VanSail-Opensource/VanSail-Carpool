package xnode

import (
	"context"
	"encoding/json"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func isUpdateCommand(name string) bool { return name == "update.install" || name == "update.rollback" }

func (s *Service) prepareUpdate(ctx context.Context, node string, operation providerops.Operation, request CommandRequest, args map[string]any) (providerops.Operation, error) {
	if !isUpdateCommand(request.Name) {
		return operation, nil
	}
	if expectation, ok := UpdateExpectationFromResult(operation.ResultJSON); ok {
		if expectation.Command != request.Name || (request.Name == "update.install" &&
			(expectation.TransactionID != operation.Receipt.ID || (request.Version != "" && expectation.Version != request.Version))) {
			return operation, ErrUpdateEvidence
		}
		if request.Name == "update.install" {
			args["version"] = expectation.Version
		}
		return operation, nil
	}
	// An old in-flight operation has no trustworthy original version. Execution
	// records it for review rather than fabricating intent or repeating mutation.
	if operation.Attempts > 0 {
		return operation, nil
	}
	if !s.master.Connected(node) || s.master.Version(node) == "" {
		return operation, &outbox.WaitError{Reason: "XNODE_UPDATE_NODE_OFFLINE", Cause: ErrUnavailable, After: time.Minute}
	}
	expectation := UpdateExpectation{Command: request.Name, TransactionID: operation.Receipt.ID, Version: request.Version, Previous: s.master.Version(node)}
	if request.Name == "update.rollback" {
		if !containsCapability(s.master.Capabilities(node), "update_transaction_v1") {
			return operation, &CapabilityError{Capability: "update_transaction_v1"}
		}
		transaction, _, err := s.master.updateTransaction(ctx, node)
		if err != nil {
			return operation, &outbox.WaitError{Reason: "XNODE_UPDATE_STATUS_PENDING", Cause: err, After: time.Minute}
		}
		if transaction.ID == "" || transaction.Version != expectation.Previous || transaction.Previous == "" ||
			transaction.Deadline.IsZero() || (transaction.Status != "pending" && transaction.Status != "confirmed") {
			return operation, ErrConflict
		}
		expectation.TransactionID, expectation.Version, expectation.Previous = transaction.ID, transaction.Version, transaction.Previous
	} else if expectation.Version == "" {
		raw, err := s.master.Command(ctx, node, "", "update.check", map[string]any{})
		if err != nil {
			return operation, &outbox.WaitError{Reason: "XNODE_UPDATE_RELEASE_LOOKUP_PENDING", Cause: err, After: time.Minute}
		}
		var release struct{ Version, Current string }
		if json.Unmarshal(raw, &release) != nil || release.Version == "" || release.Current != expectation.Previous {
			return operation, ErrConflict
		}
		expectation.Version = release.Version
	}
	if !expectation.Valid() {
		return operation, ErrInvalid
	}
	repository, ok := s.repo.(interface {
		PrepareXNodeUpdate(context.Context, string, UpdateExpectation, time.Time) (providerops.Operation, error)
	})
	if !ok {
		return operation, ErrUnavailable
	}
	prepared, err := repository.PrepareXNodeUpdate(ctx, operation.Receipt.ID, expectation, s.now().UTC())
	if err != nil {
		return operation, err
	}
	if persisted, ok := UpdateExpectationFromResult(prepared.ResultJSON); ok && request.Name == "update.install" {
		args["version"] = persisted.Version
	}
	return prepared, nil
}

// updateTransaction binds a status reply to its exact authenticated peer. It
// stays on the existing command path and never performs release-network work.
func (m *Master) updateTransaction(ctx context.Context, node string) (UpdateTransaction, *agentPeer, error) {
	m.mu.RLock()
	peer := m.peers[node]
	m.mu.RUnlock()
	if peer == nil {
		return UpdateTransaction{}, nil, ErrUnavailable
	}
	raw, err := m.commandPeer(ctx, peer, "", "update.status", map[string]any{})
	if err != nil {
		return UpdateTransaction{}, peer, err
	}
	var transaction UpdateTransaction
	if json.Unmarshal(raw, &transaction) != nil || !transaction.bounded() {
		return transaction, peer, ErrUpdateEvidence
	}
	return transaction, peer, nil
}
