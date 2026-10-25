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
		if err != nil || before.UsedBytes != after.UsedBytes {
			t.Fatal("replay repriced committed traffic", err)
		}
	}
}

func TestXNodeMissingReferencesDoNotAdvanceCursor(t *testing.T) {
	s, identity, _ := xnodeTrafficFixture(t)
	ctx := context.Background()
	at := time.Now().UTC()
	for _, item := range []struct {
		report wire.TrafficReport
		want   error
	}{
		{wire.TrafficReport{ID: "agent:1", Samples: []wire.TrafficSample{{UserID: int(identity.ProxyUserID), AccountingPeriod: "absent", From: at, Through: at, UploadBytes: 1}}}, xnode.ErrTrafficPeriodMissing},
		{wire.TrafficReport{ID: "agent:1", Traffic: wire.Traffic{999999: {1, 0}}}, xnode.ErrTrafficIdentityMissing},
	} {
		if err := s.RecordXNodeTraffic(ctx, "node", "agent", 1, "digest", item.report); !errors.Is(err, item.want) {
			t.Fatalf("got %v, want %v", err, item.want)
		}
		if sequence, err := s.XNodeReportSequence(ctx, "agent"); err != nil || sequence != 0 {
			t.Fatal("missing evidence advanced cursor", err)
		}
	}
}
