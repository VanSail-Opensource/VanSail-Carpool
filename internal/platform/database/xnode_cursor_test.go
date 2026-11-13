package database

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
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
		if err != nil || before.UsedBytes != int64(sequence)*4 || before.UsedBytes != after.UsedBytes {
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

func TestLegacyTrafficCursorCannotBeReusedByAnotherNode(t *testing.T) {
	for _, mode := range []string{"single", "batch", "rate"} {
		for _, previous := range []uint64{0, 5} {
			t.Run(fmt.Sprintf("%s/%d", mode, previous), func(t *testing.T) {
				s, identity, period := xnodeTrafficFixture(t)
				ctx, at := context.Background(), time.Now().UTC()
				if err := s.SaveXNodeNode(ctx, xnode.Node{ID: "foreign", Name: "Foreign cursor", ConfigID: "config", SettingsCipher: "encrypted"}); err != nil {
					t.Fatal(err)
				}
				if _, err := s.db.ExecContext(ctx, `INSERT INTO xnode_report_cursors(agent_id,node_id,last_sequence,last_digest,updated_at)
 VALUES('agent','foreign',$1,'retained-proof',$2)`, previous, stamp(at)); err != nil {
					t.Fatal(err)
				}
				report := wire.TrafficReport{ID: "agent:1", Samples: []wire.TrafficSample{{UserID: int(identity.ProxyUserID), AccountingPeriod: period.ID, From: at, Through: at, UploadBytes: 17}}}
				var err error
				switch mode {
				case "single":
					err = s.RecordXNodeTraffic(ctx, "node", "agent", 1, "new-proof", report)
				case "batch":
					var outcomes []error
					outcomes, err = s.RecordXNodeTrafficBatch(ctx, []xnode.TrafficCommit{{NodeID: "node", AgentID: "agent", Sequence: 1, Digest: "new-proof", Report: report}})
					if err != nil || len(outcomes) != 1 {
						t.Fatal("legacy cursor conflict was not isolated", outcomes, err)
					}
					err = outcomes[0]
				case "rate":
					err = s.SetXNodeRate(ctx, "node", "agent", previous, xnode.MultiplierScale)
				}
				if !errors.Is(err, xnode.ErrTrafficAgentMismatch) {
					t.Fatal("foreign journal ownership was acknowledged", err)
				}
				var owner, digest string
				var sequence uint64
				if err := s.db.QueryRowContext(ctx, `SELECT node_id,last_sequence,last_digest FROM xnode_report_cursors WHERE agent_id='agent'`).Scan(&owner, &sequence, &digest); err != nil || owner != "foreign" || sequence != previous || digest != "retained-proof" {
					t.Fatal("foreign cursor evidence was changed", owner, sequence, digest, err)
				}
				current, err := s.XNodePeriod(ctx, period.ID)
				if err != nil || current.UsedBytes != 0 {
					t.Fatal("rejected journal ownership changed usage", current, err)
				}
			})
		}
	}
}
