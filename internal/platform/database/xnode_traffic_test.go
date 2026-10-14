package database

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func xnodeTrafficFixture(t *testing.T) (*Store, xnode.Identity, xnode.Period) {
	t.Helper()
	store := newTestStore(t)
	ctx := context.Background()
	user := createTestUser(t, store, 80401)
	if err := store.SaveXNodeConfiguration(ctx, xnode.Configuration{ID: "config", Name: "Traffic fixture", DocumentCipher: "encrypted"}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveXNodeNode(ctx, xnode.Node{ID: "node", Name: "Traffic fixture", ConfigID: "config", Enabled: true, MultiplierFP: 1250000, SettingsCipher: "encrypted"}); err != nil {
		t.Fatal(err)
	}
	if err := store.BindXNodeAgent(ctx, "node", "agent"); err != nil {
		t.Fatal(err)
	}
	identity, err := store.SaveXNodeIdentity(ctx, xnode.Identity{UserID: user.ID, CredentialsCipher: "encrypted"})
	if err != nil {
		t.Fatal(err)
	}
	period := xnode.Period{ID: "period-one", ProxyUserID: identity.ProxyUserID, StartedAt: time.Now().UTC(), LimitBytes: 1000, Strategy: "NO_RESET", Status: "open"}
	if err = store.SaveXNodePeriod(ctx, period); err != nil {
		t.Fatal(err)
	}
	return store, identity, period
}

func TestXNodeTrafficCommitsOnceWithExactMultiplierCarry(t *testing.T) {
	store, identity, period := xnodeTrafficFixture(t)
	ctx := context.Background()
	at := time.Now().UTC()
	for sequence := uint64(1); sequence <= 4; sequence++ {
		report := wire.TrafficReport{ID: fmt.Sprintf("agent:%d", sequence), Traffic: wire.Traffic{int(identity.ProxyUserID): {1, 0}}, Samples: []wire.TrafficSample{{UserID: int(identity.ProxyUserID), AccountingPeriod: period.ID, From: at, Through: at, UploadBytes: 1}}}
		digest := sha256.Sum256([]byte(report.ID))
		for attempt := 0; attempt < 2; attempt++ {
			if err := store.RecordXNodeTraffic(ctx, "node", "agent", sequence, fmt.Sprintf("%x", digest), report); err != nil {
				t.Fatal(err)
			}
		}
	}
	value, err := store.XNodePeriod(ctx, period.ID)
	if err != nil || value.UsedBytes != 5 {
		t.Fatalf("weighted counter=%d,error=%v", value.UsedBytes, err)
	}
	rows, err := store.XNodeUsage(ctx, identity.ProxyUserID, at, at)
	if err != nil || len(rows) != 1 || rows[0].UploadBytes != 4 || rows[0].WeightedBytes != 5 {
		t.Fatalf("usage=%+v,error=%v", rows, err)
	}
}

func TestXNodeTrafficRejectsPeriodOwnerMismatchAndSequenceGaps(t *testing.T) {
	store, identity, period := xnodeTrafficFixture(t)
	ctx := context.Background()
	at := time.Now().UTC()
	report := wire.TrafficReport{ID: "agent:2", Traffic: wire.Traffic{int(identity.ProxyUserID): {4, 0}}, Samples: []wire.TrafficSample{{UserID: int(identity.ProxyUserID), AccountingPeriod: period.ID, From: at, Through: at, UploadBytes: 4}}}
	if err := store.RecordXNodeTraffic(ctx, "node", "agent", 2, "digest", report); err == nil {
		t.Fatal("sequence gap accepted")
	}
	report.ID = "agent:1"
	report.Samples[0].UserID++
	if err := store.RecordXNodeTraffic(ctx, "node", "agent", 1, "digest", report); err == nil {
		t.Fatal("foreign period charged")
	}
	sequence, err := store.XNodeReportSequence(ctx, "agent")
	if err != nil || sequence != 0 {
		t.Fatal("failed transaction acknowledged traffic")
	}
	value, err := store.XNodePeriod(ctx, period.ID)
	if err != nil || value.UsedBytes != 0 {
		t.Fatal("failed transaction changed counters")
	}
}
