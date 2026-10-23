package database

import (
	"context"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestXNodeTrafficAliasOnlyAggregateCommitsWithoutChargingAgain(t *testing.T) {
	store, identity, period := xnodeTrafficFixture(t)
	ctx := context.Background()
	if _, err := store.db.Exec(`INSERT INTO member_identity_aliases(old_user_id,user_id,old_proxy_id,proxy_id) VALUES('legacy-member',?,42,?)`, identity.UserID, identity.ProxyUserID); err != nil {
		t.Fatal(err)
	}
	// No identity row has ID 42: only the durable alias survives migration.
	report := wire.TrafficReport{ID: "agent:1", Traffic: wire.Traffic{42: {13, 17}}}
	for range 2 {
		if err := store.RecordXNodeTraffic(ctx, "node", "agent", 1, "legacy-aggregate", report); err != nil {
			t.Fatal("alias-only aggregate rejected", err)
		}
	}
	sequence, err := store.XNodeReportSequence(ctx, "agent")
	if err != nil || sequence != 1 {
		t.Fatalf("journal cursor = %d, %v", sequence, err)
	}
	value, err := store.XNodePeriod(ctx, period.ID)
	if err != nil || value.UsedBytes != 0 {
		t.Fatalf("aggregate charged without a period sample: %+v, %v", value, err)
	}
}

func TestXNodeTrafficRekeyedAggregatesDoNotReplayCumulativeCounters(t *testing.T) {
	for _, sampled := range []bool{false, true} {
		t.Run(map[bool]string{false: "idle-rekey", true: "old-period-sample"}[sampled], func(t *testing.T) {
			store, identity, period := xnodeTrafficFixture(t)
			ctx := context.Background()
			if _, err := store.db.Exec(`INSERT INTO member_identity_aliases(old_user_id,user_id,old_proxy_id,proxy_id) VALUES('legacy-member',?,42,?)`, identity.UserID, identity.ProxyUserID); err != nil {
				t.Fatal(err)
			}
			if err := store.TrackXNodePeriodNode(ctx, period.ID, "node"); err != nil {
				t.Fatal(err)
			}
			// Live cumulative totals move to the new ID; existing leases keep
			// their old period keys. Idle leases may have no new sample at all.
			report := wire.TrafficReport{ID: "agent:1", Traffic: wire.Traffic{int(identity.ProxyUserID): {9000, 8000}}}
			want := int64(0)
			if sampled {
				at := time.Now().UTC()
				report.Traffic[42] = [2]uint64{4, 0}
				report.Samples = []wire.TrafficSample{{UserID: 42, AccountingPeriod: period.ID, From: at, Through: at, UploadBytes: 4}}
				want = 5
			}
			for range 2 {
				if err := store.RecordXNodeTraffic(ctx, "node", "agent", 1, "mixed-rekey", report); err != nil {
					t.Fatal(err)
				}
			}
			sequence, err := store.XNodeReportSequence(ctx, "agent")
			if err != nil || sequence != 1 {
				t.Fatalf("cursor = %d, %v", sequence, err)
			}
			value, err := store.XNodePeriod(ctx, period.ID)
			if err != nil || value.UsedBytes != want {
				t.Fatalf("usage = %d, want %d, %v", value.UsedBytes, want, err)
			}
		})
	}
}

func TestXNodeTrafficAliasRequiresOwnerAndPeriodNodeEvidence(t *testing.T) {
	for _, scenario := range []string{"foreign-alias", "foreign-canonical-alias", "unmapped-member", "untracked-node", "tracked-node"} {
		t.Run(scenario, func(t *testing.T) {
			store, identity, period := xnodeTrafficFixture(t)
			ctx := context.Background()
			owner := identity.UserID
			if scenario == "foreign-alias" || scenario == "foreign-canonical-alias" {
				owner = "another-member"
			}
			if scenario != "unmapped-member" {
				if _, err := store.db.Exec(`INSERT INTO member_identity_aliases(old_user_id,user_id,old_proxy_id,proxy_id) VALUES('legacy-member',?,42,?)`, owner, identity.ProxyUserID); err != nil {
					t.Fatal(err)
				}
			}
			user := 42
			if scenario == "unmapped-member" || scenario == "foreign-canonical-alias" {
				user = int(identity.ProxyUserID)
			}
			report := wire.TrafficReport{ID: "agent:1", Traffic: wire.Traffic{user: {4, 0}}}
			if scenario == "untracked-node" || scenario == "tracked-node" {
				at := time.Now().UTC()
				report.Samples = []wire.TrafficSample{{UserID: user, AccountingPeriod: period.ID, From: at, Through: at, UploadBytes: 4}}
			}
			if scenario == "tracked-node" {
				if err := store.TrackXNodePeriodNode(ctx, period.ID, "node"); err != nil {
					t.Fatal(err)
				}
			}
			err := store.RecordXNodeTraffic(ctx, "node", "agent", 1, "alias-evidence", report)
			accepted := scenario == "tracked-node"
			if (err == nil) != accepted {
				t.Fatalf("accepted = %v, error = %v", accepted, err)
			}
			sequence, err := store.XNodeReportSequence(ctx, "agent")
			if err != nil || (sequence == 1) != accepted {
				t.Fatalf("journal cursor = %d, %v", sequence, err)
			}
			value, err := store.XNodePeriod(ctx, period.ID)
			want := int64(0)
			if accepted {
				want = 5 // Exact 1.25 multiplier, with no aggregate double charge.
			}
			if err != nil || value.UsedBytes != want {
				t.Fatalf("charged bytes = %d, want %d, error = %v", value.UsedBytes, want, err)
			}
		})
	}
}
