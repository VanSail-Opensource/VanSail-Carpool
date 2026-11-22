package xnode

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type firewallJournalWakeRepository struct {
	*nodeSettingsRepository
	firewallRepository
	recorded, joined int
	failure          error
}

func (r *firewallJournalWakeRepository) ReconcileFirewallMember(context.Context, string, bool, bool, uint64) error {
	r.joined++
	return nil
}

func (r *firewallJournalWakeRepository) RecordFirewallEvents(context.Context, string, string, []wire.FirewallEvent, wire.FirewallPolicy, time.Time, ...[]string) error {
	r.recorded++
	return r.failure
}

func TestFirewallJournalDoesNotWakeFleetReconciliation(t *testing.T) {
	for _, test := range []struct {
		name, kind string
		queuedWake bool
		failure    error
	}{
		{name: "expiry batch", kind: "expired"},
		{name: "source expiry", kind: "source_expired"},
		{name: "detection uses durable queue", kind: "detected"},
		{name: "existing protection wake retained", kind: "expired", queuedWake: true},
		{name: "storage failure", kind: "expired", failure: errors.New("uncommitted journal")},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, settings := encryptedNodeSettings(t, `{"firewall":{"enabled":true,"protect":true}}`)
			settings.node.Enabled = true
			repo := &firewallJournalWakeRepository{nodeSettingsRepository: settings, failure: test.failure}
			service.repo = repo
			master := NewMaster(service, nil)
			peer := &agentPeer{nodeID: "node"}
			master.peers["node"] = peer
			master.live["node"] = liveSnapshot{hello: wire.AgentHello{Capabilities: []string{"firewall_control_v1"}}}
			if test.queuedWake {
				master.NotifyFirewallChanged()
			}
			events := make([]wire.FirewallEvent, wire.MaxFirewallEventsPerFrame)
			for i := range events {
				events[i] = wire.FirewallEvent{ID: fmt.Sprintf("events-%020d", i+1), Kind: test.kind, IP: "8.8.8.8", Prefix: "8.8.8.8/32", Generation: "generation"}
			}
			message := wire.New("firewall.events", "", 0, wire.FirewallEvents{InstanceID: "events", Events: events})
			// No outbound queue deliberately exercises a lost ACK after persistence.
			// Retrying the same journal must not turn into another fleet wake.
			for attempt := 1; attempt <= 2; attempt++ {
				err := master.receiveFirewallEvents(context.Background(), peer, message)
				want := "XNODE_FIREWALL_EVENT_ACK_FAILED"
				if test.failure != nil {
					want = "XNODE_FIREWALL_EVENT_STORAGE_FAILED"
				}
				if safeFrameErrorCode(err) != want || repo.recorded != attempt || repo.joined != attempt {
					t.Fatal("journal persistence or ACK ordering changed", err, repo.recorded, repo.joined)
				}
				queued := len(master.firewallWake)
				if queued != 0 && !test.queuedWake || queued != 1 && test.queuedWake {
					t.Fatal("journal ingestion changed the fleet wake queue", queued)
				}
			}
		})
	}
}
