package xnode

import (
	"context"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestAutomaticLeaseAcceptsDisabledFirewallSourcesAndFencesReplacedPeers(t *testing.T) {
	service := NewService(nil, nil, nil, nil, nil)
	at := time.Now().UTC()
	service.now = func() time.Time { return at }
	m := NewMaster(service, nil)
	peer := &agentPeer{nodeID: "disabled-firewall-node", agentID: "agent", bootID: "boot", activityReports: true}
	m.peers[peer.nodeID] = peer
	makeReport := func(ip string, sampled time.Time) wire.Message {
		return wire.New("telemetry.report", "", 0, wire.TelemetryReport{AgentID: peer.agentID, BootID: peer.bootID, Time: at, Ready: false, Synchronized: false, Users: wire.UserMetrics{SampledAt: &sampled, ActiveIPObservations: []wire.IPActivity{{UserID: 7, IP: ip, LastActive: sampled, Observations: 1}}}})
	}
	first := makeReport("8.8.8.8", at)
	for range 3 {
		if err := m.receiveActivity(peer, first); err != nil {
			t.Fatal(err)
		}
	}
	page, _ := m.automaticLeases.page("", 1, 10, nil, at)
	if len(page.Items) != 1 || page.Items[0].ObservationCount != 1 || m.automaticLeases.generation != 1 {
		t.Fatal("disabled firewall source lost or duplicate report granted again", page)
	}
	m.peers[peer.nodeID] = &agentPeer{}
	if err := m.receiveActivity(peer, makeReport("9.9.9.9", at)); err != ErrConflict || len(m.automaticLeases.leases) != 1 {
		t.Fatal("replaced peer created a lease", err)
	}
	m.peers[peer.nodeID] = peer
	m.live[peer.nodeID] = liveSnapshot{}
	if err := m.receiveActivity(peer, first); err != nil || m.automaticLeases.generation != 1 {
		t.Fatal("reconnect duplicated the grant", err)
	}
	at = at.Add(12 * time.Hour)
	if err := m.receiveActivity(peer, makeReport("8.8.8.8", at)); err != nil || m.automaticLeases.generation != 2 {
		t.Fatal("qualifying fresh activity did not renew", err)
	}
}

func TestAutomaticLeaseSettingsPreserveLegacyWriters(t *testing.T) {
	service := NewService(nil, &globalCacheSettings{}, nil, nil, nil)
	config, err := service.Global(context.Background())
	if err != nil || config.AutomaticWhitelist == nil || !config.AutomaticWhitelist.Enabled || config.AutomaticWhitelist.DurationHours != 24 {
		t.Fatal("new defaults unavailable", config, err)
	}
	config.AutomaticWhitelist = &AutomaticWhitelistConfig{Enabled: false, DurationHours: 6}
	if err := service.SaveGlobal(context.Background(), "admin", config); err != nil {
		t.Fatal(err)
	}
	config.AutomaticWhitelist = nil
	if err := service.SaveGlobal(context.Background(), "legacy-admin", config); err != nil {
		t.Fatal(err)
	}
	latest, _ := service.Global(context.Background())
	if latest.AutomaticWhitelist.Enabled || latest.AutomaticWhitelist.DurationHours != 6 {
		t.Fatal("legacy global writer reset automatic lease settings")
	}
}
