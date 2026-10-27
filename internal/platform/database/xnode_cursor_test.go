package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestXNodeRateRacesJournalWithoutRepricingCommittedUsage(t *testing.T) {
	s, identity, period := xnodeTrafficFixture(t)
	ctx := context.Background()
	at := time.Now().UTC()
	for sequence := uint64(1); sequence <= 8; sequence++ {
		report := wire.TrafficReport{ID: fmt.Sprintf("agent:%d", sequence), Samples: []wire.TrafficSample{{UserID: int(identity.ProxyUserID), AccountingPeriod: period.ID, From: at, Through: at, UploadBytes: 4}}}
		for _, err := range runTogether(
			func() error { return s.RecordXNodeTraffic(ctx, "node", "agent", sequence, report.ID, report) },
			func() error { return s.SetXNodeRate(ctx, "node", "agent", sequence-1, int64(sequence)*1000000) },
		) {
			if err != nil {
				t.Fatal(err)
			}
		}
		before, err := s.XNodePeriod(ctx, period.ID)
		if err != nil {
			t.Fatal(err)
		}
		if err = s.RecordXNodeTraffic(ctx, "node", "agent", sequence, report.ID, report); err != nil {
			t.Fatal(err)
		}
		after, err := s.XNodePeriod(ctx, period.ID)
		if err != nil || before.UsedBytes != after.UsedBytes {
			t.Fatal("replay repriced committed traffic", err)
		}
	}
}

func TestXNodeMissingReferencesAreAuditedAndAdvanceCursor(t *testing.T) {
	for _, scenario := range []string{"period", "identity"} {
		t.Run(scenario, func(t *testing.T) {
			s, identity, _ := xnodeTrafficFixture(t)
			ctx := context.Background()
			at := time.Now().UTC()
			report := wire.TrafficReport{ID: "agent:1"}
			if scenario == "period" {
				report.Samples = []wire.TrafficSample{{UserID: int(identity.ProxyUserID), AccountingPeriod: "absent", From: at, Through: at, UploadBytes: 1}}
			} else {
				report.Traffic = wire.Traffic{999999: {1, 0}}
			}
			if err := s.RecordXNodeTraffic(ctx, "node", "agent", 1, "digest", report); err != nil {
				t.Fatal(err)
			}
			if sequence, err := s.XNodeReportSequence(ctx, "agent"); err != nil || sequence != 1 {
				t.Fatal("missing evidence blocked cursor", sequence, err)
			}
			var audits int
			if err := s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE action='xnode_traffic_discard'`).Scan(&audits); err != nil || audits != 1 {
				t.Fatal("discard was not audited", audits, err)
			}
		})
	}
}
