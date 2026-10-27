package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestTrafficPeriodAliasPreservesReportIdentityAndReplay(t *testing.T) {
	store, identity, period := xnodeTrafficFixture(t)
	ctx := context.Background()
	if _, err := store.DB().ExecContext(ctx, `INSERT INTO xnode_period_aliases(node_id,historical_period_id,canonical_period_id) VALUES(?,?,?)`, "node", "historical", period.ID); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	report := wire.TrafficReport{ID: "agent:1", Samples: []wire.TrafficSample{{UserID: int(identity.ProxyUserID), AccountingPeriod: "historical", From: at, Through: at, UploadBytes: 8}}}
	for range 2 {
		if err := store.RecordXNodeTraffic(ctx, "node", "agent", 1, "original-digest", report); err != nil {
			t.Fatal(err)
		}
	}
	if report.Samples[0].AccountingPeriod != "historical" {
		t.Fatal("source report was mutated")
	}
	got, err := store.XNodePeriod(ctx, period.ID)
	if err != nil || got.UsedBytes != 10 {
		t.Fatalf("replay double counted or missed usage: %d %v", got.UsedBytes, err)
	}
	if err = store.RecordXNodeTraffic(ctx, "node", "agent", 1, "different-digest", report); !errors.Is(err, xnode.ErrTrafficDigestMismatch) {
		t.Fatalf("changed digest accepted: %v", err)
	}
}

func TestTrafficPeriodAliasRejectsWrongOwnerAndSettledPeriod(t *testing.T) {
	for _, scenario := range []string{"owner", "closed", "unknown"} {
		t.Run(scenario, func(t *testing.T) {
			store, identity, period := xnodeTrafficFixture(t)
			ctx := context.Background()
			if _, err := store.DB().ExecContext(ctx, `INSERT INTO xnode_period_aliases(node_id,historical_period_id,canonical_period_id) VALUES(?,?,?)`, "node", "historical", period.ID); err != nil {
				t.Fatal(err)
			}
			at := time.Now().UTC()
			report := wire.TrafficReport{ID: "agent:1", Samples: []wire.TrafficSample{{UserID: int(identity.ProxyUserID), AccountingPeriod: "historical", From: at, Through: at, DownloadBytes: 4}}}
			var want error
			switch scenario {
			case "owner":
				report.Samples[0].UserID++
				want = xnode.ErrTrafficOwnerMismatch
			case "closed":
				period.Status = "closed"
				if err := store.SaveXNodePeriod(ctx, period); err != nil {
					t.Fatal(err)
				}
				want = xnode.ErrTrafficPeriodSettled
			case "unknown":
				report.Samples[0].AccountingPeriod = "unknown-period"
				want = xnode.ErrTrafficPeriodMissing
			}
			if err := store.RecordXNodeTraffic(ctx, "node", "agent", 1, "digest", report); !errors.Is(err, want) {
				t.Fatalf("error=%v want %v", err, want)
			}
			cursor, err := store.XNodeReportSequence(ctx, "agent")
			if err != nil || cursor != 0 {
				t.Fatalf("cursor advanced: %d %v", cursor, err)
			}
		})
	}
}

func TestTrafficPeriodAliasIsNodeScopedAndWholeReportRollsBack(t *testing.T) {
	store, identity, period := xnodeTrafficFixture(t)
	ctx := context.Background()
	if err := store.SaveXNodeNode(ctx, xnode.Node{ID: "other-node", Name: "Other", ConfigID: "config", Enabled: true, MultiplierFP: 1000000, SettingsCipher: "encrypted"}); err != nil {
		t.Fatal(err)
	}
	if err := store.BindXNodeAgent(ctx, "other-node", "other-agent"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `INSERT INTO xnode_period_aliases(node_id,historical_period_id,canonical_period_id) VALUES(?,?,?)`, "node", "historical", period.ID); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	sample := wire.TrafficSample{UserID: int(identity.ProxyUserID), AccountingPeriod: "historical", From: at, Through: at, UploadBytes: 8}
	report := wire.TrafficReport{ID: "other-agent:1", Samples: []wire.TrafficSample{sample}}
	if err := store.RecordXNodeTraffic(ctx, "other-node", "other-agent", 1, "digest", report); !errors.Is(err, xnode.ErrTrafficPeriodMissing) {
		t.Fatalf("other node used alias: %v", err)
	}
	report.ID = "agent:1"
	missing := sample
	missing.AccountingPeriod = "missing"
	report.Samples = append(report.Samples, missing)
	if err := store.RecordXNodeTraffic(ctx, "node", "agent", 1, "digest", report); !errors.Is(err, xnode.ErrTrafficPeriodMissing) {
		t.Fatalf("missing report reference accepted: %v", err)
	}
	got, err := store.XNodePeriod(ctx, period.ID)
	if err != nil || got.UsedBytes != 0 {
		t.Fatalf("partial report was charged: %d %v", got.UsedBytes, err)
	}
	sequence, err := store.XNodeReportSequence(ctx, "agent")
	if err != nil || sequence != 0 {
		t.Fatalf("partial report advanced cursor: %d %v", sequence, err)
	}
}
