package xnode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
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
	if !hasFirewallCapability(m.Capabilities(input.NodeID)) {
		return ErrUnavailable
	}
	if input.Bank == "protected" {
		return m.deferOperation(job, time.Minute, func(ctx context.Context) error {
			m.mu.RLock()
			peer := m.peers[input.NodeID]
			m.mu.RUnlock()
			if peer == nil {
				return ErrUnavailable
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
	err := m.deferOperation(job, timeout, func(call context.Context) error {
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
func (m *Master) deliverFirewall(ctx context.Context, node string, sources bool, job string, attempt int) error {
	repo := m.service.repo.(firewallRepository)
	m.mu.RLock()
	peer := m.peers[node]
	live := m.live[node]
	m.mu.RUnlock()
	if peer == nil || !live.report.Synchronized {
		return ErrUnavailable
	}
	desired, settings, err := m.service.Node(ctx, node)
	if err != nil {
		return err
	}
	owned, err := m.service.NativeOwner(ctx, node)
	if err != nil {
		return err
	}
	if !owned || !desired.Enabled || !settings.Firewall.Enabled {
		return nil
	}
	if containsCapability(m.Capabilities(node), "firewall_master_observation_v1") {
		if err = m.deliverProtected(ctx, peer); err != nil {
			return err
		}
	}
	member, err := repo.FirewallMember(ctx, node)
	if err != nil {
		return err
	}
	full := member.Force || member.Applied == 0
	if !sources && !full && member.Applied >= member.Version {
		return nil
	}
	if sources && !member.Force && member.SourceApplied >= member.SourceVersion {
		return nil
	}
	if sources {
		peer.firewallSourceMu.Lock()
		defer peer.firewallSourceMu.Unlock()
	}
	locked := full && !sources
	if locked {
		peer.firewallSyncMu.Lock()
		defer peer.firewallSyncMu.Unlock()
	}
	raw, err := m.Command(ctx, node, "", "firewall.bans.get", map[string]any{})
	if err != nil {
		return err
	}
	var current wire.FirewallState
	if wire.Decode(raw, &current) != nil || !current.Enabled {
		return ErrUnavailable
	}
	observed := max(current.MasterVersion, current.SourceVersion)
	if observed > member.Version {
		if err = repo.ReconcileFirewallMember(ctx, node, true, true, observed); err != nil {
			return err
		}
		member, err = repo.FirewallMember(ctx, node)
		if err != nil {
			return err
		}
		full = true
		if !locked && !sources {
			peer.firewallSyncMu.Lock()
			defer peer.firewallSyncMu.Unlock()
			locked = true
		}
	}
	identity := fmt.Sprintf("fw-%s-%d", job, attempt)
	if sources {
		return m.deliverFirewallSources(ctx, peer, member, current, identity)
	}
	cursor := ""
	index := 0
	for {
		page, next, e := repo.FirewallPage(ctx, member.Applied, member.Version, full, cursor, 256)
		if e != nil {
			return e
		}
		for _, grant := range page.Exemptions {
			if _, err := netip.ParseAddr(grant.IP); err != nil && !containsCapability(m.Capabilities(node), "firewall_cidr_exemptions_v1") {
				return &CapabilityError{Capability: "firewall_cidr_exemptions_v1"}
			}
		}
		page.SnapshotID = identity
		page.Index = index
		page.Final = next == ""
		command := "firewall.bans.update"
		if full {
			command = "firewall.bans.sync"
		} else if next != "" { // A delta must be atomic at one version: bounded pages use a full staged snapshot.
			if index == 0 && len(page.Bans)+len(page.Exemptions)+len(page.Remove)+len(page.RemoveExemptions) >= 256 {
				full = true
				cursor = ""
				if !locked {
					peer.firewallSyncMu.Lock()
					defer peer.firewallSyncMu.Unlock()
					locked = true
				}
				continue
			}
		}
		if len(page.Bans) > 0 {
			if e = repo.JoinFirewallParticipants(ctx, node, page.Bans, nil); e != nil {
				return e
			}
		}
		if _, e = m.Command(ctx, node, "", command, page); e != nil {
			return e
		}
		if next == "" {
			break
		}
		cursor = next
		index++
	}
	return repo.MarkFirewallDelivered(ctx, node, member.Version, false, full, member.Epoch)
}
