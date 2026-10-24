package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestXNodeTrafficConflictsIdentifyFailedFenceWithoutAccountingMutation(t *testing.T) {
	for _, scenario := range []string{"agent", "gap", "digest", "aggregate", "closed", "owner"} {
		t.Run(scenario, func(t *testing.T) {
			s, identity, period := xnodeTrafficFixture(t)
			ctx := context.Background()
			agent, sequence, digest := "agent", uint64(1), "new"
			var want error
			report := wire.TrafficReport{ID: "agent:1"}
			switch scenario {
			case "agent":
				agent, report.ID, want = "foreign", "foreign:1", xnode.ErrTrafficAgentMismatch
			case "gap":
				sequence, report.ID, want = 2, "agent:2", xnode.ErrTrafficSequenceGap
			case "digest":
				if err := s.RecordXNodeTraffic(ctx, "node", agent, sequence, "old", report); err != nil {
					t.Fatal(err)
				}
				want = xnode.ErrTrafficDigestMismatch
			case "aggregate":
				report.Traffic = wire.Traffic{int(identity.ProxyUserID): {1, 0}}
				want = xnode.ErrTrafficSampleMissing
			case "closed", "owner":
				at := time.Now().UTC()
				report.Samples = []wire.TrafficSample{{UserID: int(identity.ProxyUserID), AccountingPeriod: period.ID, From: at, Through: at, UploadBytes: 1}}
				if scenario == "closed" {
					period.Status = "closed"
					if err := s.SaveXNodePeriod(ctx, period); err != nil {
						t.Fatal(err)
					}
					want = xnode.ErrTrafficPeriodSettled
				} else {
					report.Samples[0].UserID++
					want = xnode.ErrTrafficOwnerMismatch
				}
			}
			if err := s.RecordXNodeTraffic(ctx, "node", agent, sequence, digest, report); !errors.Is(err, want) || !errors.Is(err, xnode.ErrConflict) {
				t.Fatalf("conflict=%v, want=%v", err, want)
			}
			cursor, err := s.XNodeReportSequence(ctx, agent)
			if err != nil || (scenario != "digest" && cursor != 0) || (scenario == "digest" && cursor != 1) {
				t.Fatal("conflict advanced cursor", cursor, err)
			}
			value, err := s.XNodePeriod(ctx, period.ID)
			if err != nil || value.UsedBytes != 0 {
				t.Fatal("conflict changed usage", value, err)
			}
		})
	}
}
