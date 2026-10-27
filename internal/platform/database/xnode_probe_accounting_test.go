package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func meteredProbeFixture(t *testing.T, store *Store, owner xnode.Identity, period xnode.Period, key string) xnode.MeteredProbe {
	t.Helper()
	ctx := context.Background()
	op, _, err := store.CreateProviderOperation(ctx, providerops.CreateInput{ActorUserID: owner.UserID, OwnerUserID: owner.UserID, Kind: xnode.RouteSpeedtestOperation, IdempotencyKey: key, RequestFingerprint: "probe-fingerprint", Items: []providerops.ItemInput{{Key: "00", TargetType: "xnode_speedtest", TargetID: "endpoint"}}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	identity, err := store.SaveXNodeIdentity(ctx, xnode.Identity{ServiceName: "member-probe-" + key, CredentialsCipher: "encrypted"})
	if err != nil {
		t.Fatal(err)
	}
	p := xnode.MeteredProbe{ProxyUserID: identity.ProxyUserID, PeriodID: period.ID, OperationID: op.Receipt.ID, PairKey: "00", EndpointID: "endpoint", NodeID: "node", InboundTag: "paid", ValidUntil: time.Now().Add(time.Minute)}
	if err = store.SaveXNodeMeteredProbe(ctx, p); err != nil {
		t.Fatal(err)
	}
	return p
}

func recordProbeSample(t *testing.T, store *Store, probe xnode.MeteredProbe, sequence uint64, bytes uint64) error {
	t.Helper()
	at := time.Now().UTC()
	report := wire.TrafficReport{ID: fmt.Sprintf("agent:%d", sequence), Traffic: wire.Traffic{int(probe.ProxyUserID): {bytes, 0}}, Samples: []wire.TrafficSample{{UserID: int(probe.ProxyUserID), AccountingPeriod: probe.PeriodID, From: at, Through: at, UploadBytes: bytes}}}
	return store.RecordXNodeTraffic(context.Background(), "node", "agent", sequence, fmt.Sprint(sequence), report)
}

func TestMeteredProbeSamplesDeduplicateAndCarryMultiplier(t *testing.T) {
	store, owner, period := xnodeTrafficFixture(t)
	probe := meteredProbeFixture(t, store, owner, period, "dedup")
	for sequence := uint64(1); sequence <= 4; sequence++ {
		for duplicate := 0; duplicate < 2; duplicate++ {
			if err := recordProbeSample(t, store, probe, sequence, 1); err != nil {
				t.Fatal(err)
			}
		}
	}
	value, err := store.XNodePeriod(context.Background(), period.ID)
	if err != nil || value.UsedBytes != 5 {
		t.Fatalf("owner quota: %#v, %v", value, err)
	}
	usage, err := store.XNodeUsage(context.Background(), owner.ProxyUserID, time.Now(), time.Now())
	if err != nil || len(usage) != 1 || usage[0].WeightedBytes != 5 {
		t.Fatalf("owner history: %#v, %v", usage, err)
	}
	usage, err = store.XNodeUsage(context.Background(), probe.ProxyUserID, time.Now(), time.Now())
	if err != nil || len(usage) != 0 {
		t.Fatalf("probe double-charge: %#v, %v", usage, err)
	}
}

func TestMeteredProbeMissingPeriodAndWrongTargetAreDiscarded(t *testing.T) {
	store, owner, period := xnodeTrafficFixture(t)
	p := meteredProbeFixture(t, store, owner, period, "invalid")
	report := wire.TrafficReport{ID: "agent:1", Traffic: wire.Traffic{int(p.ProxyUserID): {8, 0}}}
	if err := store.RecordXNodeTraffic(context.Background(), "node", "agent", 1, "digest", report); err != nil {
		t.Fatal("unperioded traffic discard failed", err)
	}
	p.PeriodID = "not-owned"
	if err := recordProbeSample(t, store, p, 2, 8); err != nil {
		t.Fatal("foreign period discard failed", err)
	}
	sequence, err := store.XNodeReportSequence(context.Background(), "agent")
	if err != nil || sequence != 2 {
		t.Fatal("discard did not advance", sequence, err)
	}
	value, err := store.XNodePeriod(context.Background(), period.ID)
	if err != nil || value.UsedBytes != 0 {
		t.Fatal("discard charged unmatched traffic", value, err)
	}
}

func TestMeteredProbeOriginSurvivesResetRenewalAndRefundSettlement(t *testing.T) {
	for _, action := range []string{"reset", "renewal", "refund"} {
		t.Run(action, func(t *testing.T) {
			ctx := context.Background()
			store, owner, period := xnodeTrafficFixture(t)
			p := meteredProbeFixture(t, store, owner, period, action)
			period.Status = "closing"
			if err := store.SaveXNodePeriod(ctx, period); err != nil {
				t.Fatal(err)
			}
			successor := xnode.Period{ID: "paid-successor", ProxyUserID: owner.ProxyUserID, StartedAt: time.Now(), Status: "open", Strategy: "NO_RESET", LimitBytes: 1000}
			if err := store.SaveXNodePeriod(ctx, successor); err != nil {
				t.Fatal(err)
			}
			if err := store.RevokeXNodeMeteredProbe(ctx, p.ProxyUserID, time.Now()); err != nil {
				t.Fatal(err)
			}
			// Partial/uncertain transfers still arrive after revocation and must be
			// billed once to their origin, never to the newly active successor.
			if err := recordProbeSample(t, store, p, 1, 12); err != nil {
				t.Fatal(err)
			}
			old, err := store.XNodePeriod(ctx, period.ID)
			if err != nil || old.UsedBytes != 15 {
				t.Fatal("origin lost", old, err)
			}
			next, err := store.XNodePeriod(ctx, successor.ID)
			if err != nil || next.UsedBytes != 0 || next.Status != "open" {
				t.Fatal("successor changed", next, err)
			}
			if err := store.SealXNodeMeteredProbe(ctx, p.ProxyUserID, 1); err != nil {
				t.Fatal(err)
			}
			period.Status = "closed"
			if err := store.SaveXNodePeriod(ctx, period); err != nil {
				t.Fatal(err)
			}
			if err := recordProbeSample(t, store, p, 2, 1); err != nil {
				t.Fatal("settled traffic discard failed", err)
			}
			old, err = store.XNodePeriod(ctx, period.ID)
			if err != nil || old.UsedBytes != 15 {
				t.Fatal("settled financial period mutated", old, err)
			}
			p.ProxyUserID++
			if err := store.SaveXNodeMeteredProbe(ctx, p); err == nil {
				t.Fatal("new grant to ended origin")
			}
		})
	}
}
