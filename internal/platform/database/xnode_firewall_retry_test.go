package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestFirewallDetectionRetriesResetUncommittedWindows(t *testing.T) {
	t.Parallel()
	for _, initialCount := range []uint64{2, 3} {
		t.Run(fmt.Sprint(initialCount), func(t *testing.T) {
			t.Parallel()
			store, _, _ := xnodeTrafficFixture(t)
			ctx := context.Background()
			now := time.Now().UTC()
			if err := store.ReconcileFirewallMember(ctx, "node", true, true, 0); err != nil {
				t.Fatal(err)
			}
			for _, query := range []string{
				`CREATE SEQUENCE firewall_retry_attempt`,
				`CREATE FUNCTION retry_firewall_cursor_once() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
				IF nextval('firewall_retry_attempt')=1 THEN RAISE EXCEPTION 'injected retry after staging' USING ERRCODE='40001'; END IF;
				RETURN NEW; END $$`,
				`CREATE TRIGGER retry_firewall_cursor_once BEFORE INSERT ON xnode_firewall_cursors
				FOR EACH ROW EXECUTE FUNCTION retry_firewall_cursor_once()`,
			} {
				if _, err := store.DB().ExecContext(ctx, query); err != nil {
					t.Fatal(err)
				}
			}
			policy := wire.FirewallPolicy{Enabled: true, Protect: true,
				Rules: map[string]wire.FirewallRule{"closed_port": {Enabled: true, Threshold: 3, WindowSeconds: 60}}}
			event := wire.FirewallEvent{ID: fmt.Sprintf("retry-events-%020d", 1), Kind: "detected",
				IP: "8.8.8.8", Rule: "closed_port", Count: initialCount, FirstSeen: now, Time: now}
			if err := store.RecordFirewallEvents(ctx, "node", "retry-events", []wire.FirewallEvent{event}, policy, now); err != nil {
				t.Fatal(err)
			}
			var attempts int
			if err := store.DB().QueryRowContext(ctx, `SELECT last_value FROM firewall_retry_attempt`).Scan(&attempts); err != nil || attempts != 2 {
				t.Fatalf("injected transaction attempts = %d, %v", attempts, err)
			}
			for range 2 {
				bans, err := store.FirewallBans(ctx, 10)
				want := 0
				if initialCount == 3 {
					want = 1
				}
				if err != nil || len(bans) != want || len(bans) == 1 && bans[0].Count != "3" {
					t.Fatalf("replayed detection lost or multiplied the delta: %+v, %v", bans, err)
				}
				if err := store.RecordFirewallEvents(ctx, "node", "retry-events", []wire.FirewallEvent{event}, policy, now); err != nil {
					t.Fatal(err)
				}
			}
			event.ID, event.Count = fmt.Sprintf("retry-events-%020d", 2), 1
			event.Time = now.Add(time.Second)
			if err := store.RecordFirewallEvents(ctx, "node", "retry-events", []wire.FirewallEvent{event}, policy, event.Time); err != nil {
				t.Fatal(err)
			}
			bans, err := store.FirewallBans(ctx, 10)
			if err != nil || len(bans) != 1 || bans[0].Count != "3" {
				t.Fatalf("committed window did not produce exactly one threshold ban: %+v, %v", bans, err)
			}
		})
	}
}
