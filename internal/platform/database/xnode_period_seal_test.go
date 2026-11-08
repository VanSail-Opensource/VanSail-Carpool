package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestParticipantSealRejectsLegacyCursorOwnedByAnotherNode(t *testing.T) {
	for _, tc := range []struct {
		name   string
		stream bool
		legacy uint64
	}{{"legacy-positive", false, 1}, {"legacy-zero", false, 0}, {"stream-positive", true, 1}, {"stream-zero", true, 0}} {
		t.Run(tc.name, func(t *testing.T) {
			s, _, period := xnodeTrafficFixture(t)
			ctx := context.Background()
			foreign := xnode.Node{ID: "foreign", Name: "Other", ConfigID: "config", SettingsCipher: "encrypted", MultiplierFP: xnode.MultiplierScale}
			if err := s.SaveXNodeNode(ctx, foreign); err != nil {
				t.Fatal(err)
			}
			if err := s.TrackXNodePeriodNode(ctx, period.ID, "node"); err != nil {
				t.Fatal(err)
			}
			barrier := wire.TrafficBarrier{AgentID: "agent", AccountingPeriod: period.ID, Quiesced: true, Sequence: tc.legacy}
			if tc.stream {
				barrier.StreamID, barrier.Sequence, barrier.LegacySequence = "11111111111111111111111111111111", 0, tc.legacy
				if err := s.RegisterXNodeTrafficStream(ctx, "node", "agent", barrier.StreamID); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := s.DB().ExecContext(ctx, `INSERT INTO xnode_report_cursors(agent_id,node_id,last_sequence,last_digest,updated_at) VALUES('agent','foreign',$1,'fixture',$2)`, tc.legacy, stamp(time.Now())); err != nil {
				t.Fatal(err)
			}
			period, err := s.BeginXNodePeriodClosure(ctx, period, time.Now().UTC())
			if err != nil {
				t.Fatal(err)
			}
			if err = s.SealXNodeClosingPeriodNode(ctx, period, "node", barrier); !errors.Is(err, xnode.ErrConflict) {
				t.Fatal("foreign cursor authorized participant seal", err)
			}
			participants, err := s.XNodePeriodNodes(ctx, period.ID)
			if err != nil || len(participants) != 1 || participants[0].Flushed {
				t.Fatal("rejected evidence changed accounting", participants, err)
			}
			if _, err = s.DB().ExecContext(ctx, `UPDATE xnode_report_cursors SET node_id='node' WHERE agent_id='agent'`); err != nil {
				t.Fatal(err)
			}
			if err = s.SealXNodeClosingPeriodNode(ctx, period, "node", barrier); err != nil {
				t.Fatal("matching committed evidence rejected", err)
			}
		})
	}
}

func TestReopenClearsMerelyDisabledNodeProof(t *testing.T) {
	s, _, period := xnodeTrafficFixture(t)
	ctx := context.Background()
	if err := s.TrackXNodePeriodNode(ctx, period.ID, "node"); err != nil {
		t.Fatal(err)
	}
	period, err := s.BeginXNodePeriodClosure(ctx, period, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SealXNodeClosingPeriodNode(ctx, period, "node", wire.TrafficBarrier{AgentID: "agent", AccountingPeriod: period.ID, Quiesced: true}); err != nil {
		t.Fatal(err)
	}
	node, err := s.XNodeNode(ctx, "node")
	if err != nil {
		t.Fatal(err)
	}
	node.Enabled = false
	if err = s.SaveXNodeNode(ctx, node); err != nil {
		t.Fatal(err)
	}
	if err = s.ReopenXNodePeriod(ctx, period.ID); err != nil {
		t.Fatal(err)
	}
	participants, err := s.XNodePeriodNodes(ctx, period.ID)
	if err != nil || len(participants) != 1 || participants[0].Flushed {
		t.Fatal("ordinary disabled node retained an obsolete barrier", participants, err)
	}
}
