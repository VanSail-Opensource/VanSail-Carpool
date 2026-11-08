package database

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestReopenRetainsRetiredProofAndStillWaitsForUnfencedOfflineNode(t *testing.T) {
	for _, retire := range []bool{false, true} {
		t.Run(map[bool]string{false: "removing", true: "retired"}[retire], func(t *testing.T) {
			checkReopenedRetirementProof(t, retire)
		})
	}
}

func checkReopenedRetirementProof(t *testing.T, retire bool) {
	t.Helper()
	s, identity, period := xnodeTrafficFixture(t)
	ctx := context.Background()
	active := xnode.Node{ID: "active", Name: "Offline participant", Enabled: true, ConfigID: "config", MultiplierFP: xnode.MultiplierScale, SettingsCipher: "encrypted"}
	if err := s.SaveXNodeNode(ctx, active); err != nil {
		t.Fatal(err)
	}
	if err := s.BindXNodeAgent(ctx, active.ID, "other-agent"); err != nil {
		t.Fatal(err)
	}
	for _, node := range []string{"node", active.ID} {
		if err := s.TrackXNodePeriodNode(ctx, period.ID, node); err != nil {
			t.Fatal(err)
		}
	}
	const stream = "11111111111111111111111111111111"
	if err := s.RegisterXNodeTrafficStream(ctx, "node", "agent", stream); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	for _, boot := range []string{stream, ""} {
		id := "agent:1"
		if boot != "" {
			id = "agent:" + boot + ":1"
		}
		report := wire.TrafficReport{ID: id, StreamID: boot, Samples: []wire.TrafficSample{{UserID: int(identity.ProxyUserID), AccountingPeriod: period.ID, From: at, Through: at, UploadBytes: 1}}}
		if err := s.RecordXNodeTraffic(ctx, "node", "agent", 1, id, report); err != nil {
			t.Fatal(err)
		}
	}
	version, _, err := s.XNodeCompileVersion(ctx, at)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.PrepareXNodeRemoval(ctx, "node", version); err != nil {
		t.Fatal(err)
	}
	retiredBarrier := wire.TrafficBarrier{AgentID: "agent", StreamID: stream, Sequence: 1, LegacySequence: 1, AccountingPeriod: period.ID, Quiesced: true}
	if err = s.SealXNodeRemovalPeriodNode(ctx, period, "node", retiredBarrier); err != nil {
		t.Fatal(err)
	}
	version, _, err = s.XNodeCompileVersion(ctx, at)
	if err != nil {
		t.Fatal(err)
	}
	if retire {
		if retired, err := s.FinishXNodeRemoval(ctx, "node", version, identity.UserID, "retire-before-reopen"); err != nil || !retired {
			t.Fatal(retired, err)
		}
	}
	period, err = s.BeginXNodePeriodClosure(ctx, period, at)
	if err != nil {
		t.Fatal(err)
	}
	activeBarrier := wire.TrafficBarrier{AgentID: "other-agent", AccountingPeriod: period.ID, Quiesced: true}
	if err = s.SealXNodeClosingPeriodNode(ctx, period, active.ID, activeBarrier); err != nil {
		t.Fatal(err)
	}
	if err = s.FinalizeXNodePeriod(ctx, period); err != nil {
		t.Fatal(err)
	}
	if err = s.ReopenXNodePeriod(ctx, period.ID); err != nil {
		t.Fatal(err)
	}
	var sequence, legacy uint64
	var retainedStream string
	err = s.DB().QueryRowContext(ctx, `SELECT flushed_sequence,flushed_stream_id,flushed_legacy_sequence FROM xnode_period_nodes WHERE period_id=$1 AND node_id='node'`, period.ID).Scan(&sequence, &retainedStream, &legacy)
	if err != nil || sequence != 1 || retainedStream != stream || legacy != 1 {
		t.Fatal("retired drain evidence was lost", sequence, retainedStream, legacy, err)
	}
	period, err = s.XNodePeriod(ctx, period.ID)
	if err != nil {
		t.Fatal(err)
	}
	period, err = s.BeginXNodePeriodClosure(ctx, period, at)
	if err != nil {
		t.Fatal(err)
	}
	service := xnode.NewService(s, nil, nil, nil, nil)
	err = service.ResumeAccounting(ctx)
	if !errors.Is(err, xnode.ErrUnavailable) || !strings.Contains(err.Error(), "node active:") || strings.Contains(err.Error(), "node node:") {
		t.Fatal("retired node was recontacted or genuine offline node skipped", err)
	}
	if err = s.FinalizeXNodePeriod(ctx, period); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("unfenced offline participant was skipped", err)
	}
	if err = s.SealXNodeClosingPeriodNode(ctx, period, active.ID, activeBarrier); err != nil {
		t.Fatal(err)
	}
	if err = s.FinalizeXNodePeriod(ctx, period); err != nil {
		t.Fatal(err)
	}
}

func TestRemovalSealerRequiresPermanentFenceOwnerAndCommittedCursor(t *testing.T) {
	s, _, period := xnodeTrafficFixture(t)
	ctx := context.Background()
	if err := s.TrackXNodePeriodNode(ctx, period.ID, "node"); err != nil {
		t.Fatal(err)
	}
	barrier := wire.TrafficBarrier{AgentID: "agent", AccountingPeriod: period.ID, Quiesced: true}
	if err := s.SealXNodeRemovalPeriodNode(ctx, period, "node", barrier); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("unfenced node sealed", err)
	}
	node, err := s.XNodeNode(ctx, "node")
	if err != nil {
		t.Fatal(err)
	}
	node.Enabled = false
	if err = s.SaveXNodeNode(ctx, node); err != nil {
		t.Fatal(err)
	}
	if err = s.SealXNodeRemovalPeriodNode(ctx, period, "node", barrier); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("ordinary disabled node counted as permanently revoked", err)
	}
	version, _, err := s.XNodeCompileVersion(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.PrepareXNodeRemoval(ctx, "node", version); err != nil {
		t.Fatal(err)
	}
	for _, altered := range []string{"owner", "purchase", "agent", "cursor"} {
		p, b := period, barrier
		switch altered {
		case "owner":
			p.ProxyUserID++
		case "purchase":
			p.PurchaseID = "unrelated"
		case "agent":
			b.AgentID = "unrelated"
		case "cursor":
			b.Sequence = 1
		}
		if err = s.SealXNodeRemovalPeriodNode(ctx, p, "node", b); !errors.Is(err, xnode.ErrConflict) {
			t.Fatal("invalid removal evidence accepted", altered, err)
		}
	}
	closing, err := s.BeginXNodePeriodClosure(ctx, period, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.ReopenXNodePeriod(ctx, period.ID); err != nil {
		t.Fatal(err)
	}
	closing.Status = "open"
	closing, err = s.BeginXNodePeriodClosure(ctx, closing, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.FinalizeXNodePeriod(ctx, closing); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("unconfirmed removal proof survived reopen", err)
	}
	if err = s.SealXNodeRemovalPeriodNode(ctx, period, "node", barrier); err != nil {
		t.Fatal(err)
	}
	if err = s.FinalizeXNodePeriod(ctx, closing); err != nil {
		t.Fatal(err)
	}
}
