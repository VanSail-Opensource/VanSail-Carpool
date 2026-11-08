package xnode

import (
	"context"
	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
	"time"
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
			return m.deliverProtected(ctx, peer)
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
			return m.deliverCountry(call, input.NodeID)
		}
		return m.deliverFirewall(call, input.NodeID, input.Bank == "sources", job.ID, job.Attempts)
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
