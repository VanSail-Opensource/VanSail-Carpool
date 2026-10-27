package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestXNodeTrafficConflictsAreAuditedWithoutAccountingMutation(t *testing.T) {
	for _, scenario := range []string{"agent", "gap", "digest", "aggregate", "closed", "owner"} {
		t.Run(scenario, func(t *testing.T) {
			s, identity, period := xnodeTrafficFixture(t)
			ctx := context.Background()
			agent, sequence, digest := "agent", uint64(1), "new"
			report := wire.TrafficReport{ID: "agent:1"}
			switch scenario {
			case "agent":
				agent, report.ID = "foreign", "foreign:1"
			case "gap":
				sequence, report.ID = 2, "agent:2"
			case "digest":
				if err := s.RecordXNodeTraffic(ctx, "node", agent, sequence, "old", report); err != nil {
					t.Fatal(err)
				}
			case "aggregate":
				report.Traffic = wire.Traffic{int(identity.ProxyUserID): {1, 0}}
			case "closed", "owner":
				at := time.Now().UTC()
				report.Samples = []wire.TrafficSample{{UserID: int(identity.ProxyUserID), AccountingPeriod: period.ID, From: at, Through: at, UploadBytes: 1}}
				if scenario == "closed" {
					period.Status = "closed"
					if err := s.SaveXNodePeriod(ctx, period); err != nil {
						t.Fatal(err)
					}
				} else {
					report.Samples[0].UserID++
				}
			}
			err := s.RecordXNodeTraffic(ctx, "node", agent, sequence, digest, report)
			if scenario == "agent" {
				if !errors.Is(err, xnode.ErrTrafficAgentMismatch) {
					t.Fatalf("foreign agent accepted: %v", err)
				}
			} else if err != nil {
				t.Fatalf("discard failed: %v", err)
			}
			if scenario == "digest" {
				if err := s.RecordXNodeTraffic(ctx, "node", agent, sequence, digest, report); err != nil {
					t.Fatalf("duplicate discard failed: %v", err)
				}
			}
			cursor, err := s.XNodeReportSequence(ctx, "agent")
			wantCursor := sequence
			if scenario == "agent" {
				wantCursor = 0
			}
			if err != nil || cursor != wantCursor {
				t.Fatal("wrong cursor", cursor, err)
			}
			value, err := s.XNodePeriod(ctx, period.ID)
			if err != nil || value.UsedBytes != 0 {
				t.Fatal("conflict changed usage", value, err)
			}
			var audits int
			if err = s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE action='xnode_traffic_discard'`).Scan(&audits); err != nil || (audits == 0) != (scenario == "agent") {
				t.Fatal("wrong audit count", audits, err)
			}
		})
	}
}
