package xnode

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
)

func (m *Master) HandleOutbox(ctx context.Context, job model.OutboxJob) error {
	if job.Kind == PAYGJob {
		return m.service.ReconcilePAYG(ctx, job)
	}
	if job.Kind == PAYGRecoveryJob {
		return m.deferOperation(job, 2*time.Minute, func(call context.Context) error { return m.recoverPAYG(call, job) })
	}
	if job.Kind == DIYReconcileJob {
		return m.service.ReconcileDIY(ctx, job)
	}
	if job.Kind == AccessRepairJob {
		return m.deferOperation(job, 5*time.Minute, func(call context.Context) error {
			return m.service.RepairAccess(call, job)
		})
	}
	var input struct {
		NodeID string `json:"nodeId"`
		Bank   string `json:"bank"`
	}
	if json.Unmarshal([]byte(job.Payload), &input) != nil || input.NodeID == "" || (input.Bank != "dynamic" && input.Bank != "sources" && input.Bank != "geoip" && input.Bank != "protected") {
		return ErrInvalid
	}
	node, settings, err := m.service.Node(ctx, input.NodeID)
	if isMissing(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if node.RetiredAt != nil || input.Bank != "protected" && (!node.Enabled || !settings.Firewall.Enabled) {
		return nil // Durable removal/disable supersedes this delivery intent.
	}
	if !m.Connected(input.NodeID) {
		return &outbox.WaitError{Reason: "firewall_node_offline", Cause: ErrUnavailable, After: time.Minute}
	}
	if !hasFirewallCapability(m.Capabilities(input.NodeID)) {
		return &outbox.WaitError{Reason: "firewall_capability_pending", Cause: ErrUnavailable, After: time.Minute}
	}
	if input.Bank == "protected" {
		return m.deferOperation(job, time.Minute, func(ctx context.Context) error {
			m.mu.RLock()
			peer := m.peers[input.NodeID]
			m.mu.RUnlock()
			if peer == nil {
				return &outbox.WaitError{Reason: "firewall_node_offline", Cause: ErrUnavailable, After: time.Minute}
			}
			return firewallDeliveryWait(m.deliverProtected(ctx, peer))
		})
	}
	if input.Bank != "dynamic" {
		m.mu.Lock()
		if m.firewallSourcesRunning >= 2 {
			m.mu.Unlock()
			return outbox.ErrBusy
		}
		m.firewallSourcesRunning++
		m.mu.Unlock()
	}
	timeout := 5 * time.Minute
	if input.Bank != "dynamic" {
		timeout = 30 * time.Minute
	}
	err = m.deferOperation(job, timeout, func(call context.Context) error {
		if input.Bank != "dynamic" {
			defer func() { m.mu.Lock(); m.firewallSourcesRunning--; m.mu.Unlock() }()
		}
		if input.Bank == "geoip" {
			return firewallDeliveryWait(m.deliverCountry(call, input.NodeID))
		}
		return firewallDeliveryWait(m.deliverFirewall(call, input.NodeID, input.Bank == "sources", job.ID, job.Attempts))
	})
	if err != nil && input.Bank != "dynamic" { // ErrDeferred owns the slot until completion.
		if !isDeferredFirewall(err) {
			m.mu.Lock()
			m.firewallSourcesRunning--
			m.mu.Unlock()
		}
	}
	return err
}

// firewallDeliveryWait classifies an idempotent desired-state firewall delivery
// that could not be reached or acknowledged as a dependency wait. Attempts are
// not consumed while a node is offline or reconnecting, so a long outage cannot
// exhaust the durable delivery into a terminal failed row. A rejection or a
// command the agent executed and reported as failed stays an ordinary error.
func firewallDeliveryWait(err error) error {
	if err == nil {
		return nil
	}
	var failure *CommandError
	if errors.As(err, &failure) && (failure.Outcome == CommandUndelivered || failure.Outcome == CommandUncertain) {
		if errors.Is(failure.Cause, ErrUnavailable) || errors.Is(failure.Cause, context.Canceled) || errors.Is(failure.Cause, context.DeadlineExceeded) {
			return &outbox.WaitError{Reason: "firewall_node_offline", Cause: ErrUnavailable, After: time.Minute}
		}
	}
	return err
}
