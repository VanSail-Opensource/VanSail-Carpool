package database

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestDDNSHostMutationFamiliesFencesAndRecovery(t *testing.T) {
	for _, family := range []string{"ipv4", "ipv6", "dual"} {
		t.Run(family, func(t *testing.T) {
			s, in := ddnsChangeFixture(t)
			ctx := context.Background()
			now := time.Now().UTC()
			for _, host := range []xnode.Endpoint{{ID: "ddns-v4", NodeID: in.Node.ID, Name: "v4", InboundTag: "paid", Port: 443, Address: "203.0.113.7", Enabled: true}, {ID: "ddns-v6", NodeID: in.Node.ID, Name: "v6", InboundTag: "paid", Port: 443, Address: "2001:db8::7", Enabled: true}} {
				if err := s.SaveXNodeEndpoint(ctx, host); err != nil {
					t.Fatal(err)
				}
			}
			if family != "ipv6" {
				in.Binding.IPv4HostIDs = []string{"ddns-v4"}
			} else {
				in.Change.IPv4 = ""
			}
			if family != "ipv4" {
				in.Binding.IPv6HostIDs = []string{"ddns-v6"}
				in.Change.IPv6 = "2001:db8::8"
			}
			if err := s.SaveXNodeNodeDDNS(ctx, in.Node, map[string]xnode.DDNSHostBinding{"wan": in.Binding}); err != nil {
				t.Fatal(err)
			}
			version, _, err := s.XNodeCompileVersion(ctx, now)
			if err != nil {
				t.Fatal(err)
			}
			in.Version = version
			if _, err = s.AcceptXNodeDDNSChange(ctx, in, now); err != nil {
				t.Fatal(err)
			}
			states, err := s.XNodeDDNSChanges(ctx)
			if err != nil {
				t.Fatal(err)
			}
			state := states[0]
			stale := in
			stale.Version--
			if err = s.ApplyXNodeDDNSChange(ctx, state, stale, now); err == nil {
				t.Fatal("stale epoch changed hosts")
			}
			if err = s.ApplyXNodeDDNSChange(ctx, state, in, now); err != nil {
				t.Fatal(err)
			}
			if err = s.ApplyXNodeDDNSChange(ctx, state, in, now); err == nil {
				t.Fatal("old phase replay applied twice")
			}
			hosts, err := s.XNodeEndpoints(ctx)
			if err != nil {
				t.Fatal(err)
			}
			for _, host := range hosts {
				expected := "203.0.113.7"
				if host.ID == "ddns-v6" {
					expected = "2001:db8::7"
				}
				if slices.Contains(in.Binding.IPv4HostIDs, host.ID) {
					expected = in.Change.IPv4
				}
				if slices.Contains(in.Binding.IPv6HostIDs, host.ID) {
					expected = in.Change.IPv6
				}
				if host.Address != expected || host.InboundTag != "paid" || host.Port != 443 || !host.Enabled {
					t.Fatal("unrelated host fields changed", host)
				}
			}
			version, _, err = s.XNodeCompileVersion(ctx, now)
			if err != nil || version <= in.Version {
				t.Fatal("host change did not invalidate compilation", version, err)
			}
			states, err = s.XNodeDDNSChanges(ctx)
			if err != nil || states[0].Phase != "routes" {
				t.Fatal(states, err)
			}
			if _, err = s.DB().ExecContext(ctx, `UPDATE outbox_jobs SET status='failed',attempts=10 WHERE kind='provider_operation'`); err != nil {
				t.Fatal(err)
			}
			if err = s.WakeXNodeRouting(ctx, now); err != nil {
				t.Fatal(err)
			}
			var status string
			var attempts int
			if err = s.DB().QueryRowContext(ctx, `SELECT status,attempts FROM outbox_jobs WHERE kind='provider_operation'`).Scan(&status, &attempts); err != nil || status != "pending" || attempts != 0 {
				t.Fatal("reconnect did not recover desired DDNS", status, attempts, err)
			}
			if err = s.AdvanceXNodeDDNSChange(ctx, states[0], "settled", []string{}); err != nil {
				t.Fatal(err)
			}
			saved, err := s.XNodeNode(ctx, in.Node.ID)
			if err != nil || saved.SettingsCipher != in.Node.SettingsCipher || saved.AgentID != in.Node.AgentID {
				t.Fatal("discovery credentials changed", saved, err)
			}
			var debits int
			if err = s.DB().QueryRowContext(ctx, `SELECT count(*) FROM ledger_entries`).Scan(&debits); err != nil || debits != 0 {
				t.Fatal("automatic DDNS charged money", debits, err)
			}
		})
	}
}
