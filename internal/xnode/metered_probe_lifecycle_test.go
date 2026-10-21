package xnode_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestMeteredProbeCompiledAccessQuotaExpiryAndRealReset(t *testing.T) {
	for _, action := range []string{"quota", "expired", "reset", "refund-quiesce"} {
		t.Run(action, func(t *testing.T) {
			f := controlFixture(t, true)
			ctx := context.Background()
			owner, err := f.store.XNodeIdentity(ctx, f.identity.ProxyUserID)
			if err != nil {
				t.Fatal(err)
			}
			period, err := f.store.XNodePeriod(ctx, owner.CurrentPeriodID)
			if err != nil {
				t.Fatal(err)
			}
			period.LimitBytes = 10
			if err = f.store.SaveXNodePeriod(ctx, period); err != nil {
				t.Fatal(err)
			}
			// This reserved identity has no admission until the immutable metering
			// reference exists, even though generic service credentials are valid.
			identity, err := f.service.AuthorizeService(ctx, "member-probe-hosted-"+action, xnode.Endpoint{NodeID: f.node.ID, InboundTag: "unused"}, time.Now().Add(time.Minute))
			if err != nil {
				t.Fatal(err)
			}
			op, _, err := f.store.CreateProviderOperation(ctx, providerops.CreateInput{ActorUserID: f.user.ID, OwnerUserID: f.user.ID, Kind: xnode.RouteSpeedtestOperation, IdempotencyKey: action, RequestFingerprint: action, Items: []providerops.ItemInput{{Key: "00", TargetType: "xnode_speedtest", TargetID: "synthetic"}}}, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			until := time.Now().Add(time.Minute)
			if action == "expired" {
				until = time.Now().Add(-time.Second)
			}
			probe := xnode.MeteredProbe{ProxyUserID: identity.ProxyUserID, PeriodID: period.ID, OperationID: op.Receipt.ID, PairKey: "00", EndpointID: "target", NodeID: f.node.ID, InboundTag: "paid", ValidUntil: until}
			if err = f.store.SaveXNodeMeteredProbe(ctx, probe); err != nil {
				t.Fatal(err)
			}
			compiled, err := f.service.Compile(ctx, f.node.ID)
			if err != nil {
				t.Fatal(err)
			}
			present := false
			for _, user := range compiled.Users {
				if user.ID == int(probe.ProxyUserID) {
					present = true
					if user.AccountingPeriod != period.ID || len(user.InboundTags) != 1 || user.InboundTags[0] != "paid" {
						t.Fatal("unrestricted probe", user.ID)
					}
				}
			}
			if present != (action != "expired") {
				t.Fatal("probe expiry admission", present)
			}
			if err = f.store.BindXNodeAgent(ctx, f.node.ID, "metered-agent"); err != nil {
				t.Fatal(err)
			}
			if err = f.store.TrackXNodePeriodNode(ctx, period.ID, f.node.ID); err != nil {
				t.Fatal(err)
			}
			if action == "reset" {
				if err = f.service.ResetTraffic(ctx, stringID(owner)); err != nil {
					t.Fatal(err)
				}
			} else if action == "refund-quiesce" {
				if err = f.service.Quiesce(ctx, stringID(owner)); !errors.Is(err, xnode.ErrUnavailable) {
					t.Fatal("refund settled without probe barrier", err)
				}
			}
			at := time.Now().UTC()
			report := wire.TrafficReport{ID: "metered-agent:1", Samples: []wire.TrafficSample{{UserID: int(probe.ProxyUserID), AccountingPeriod: period.ID, From: at, Through: at, UploadBytes: 12}}}
			for duplicate := 0; duplicate < 2; duplicate++ {
				if err = f.store.RecordXNodeTraffic(ctx, f.node.ID, "metered-agent", 1, "same-tail", report); err != nil {
					t.Fatal(err)
				}
			}
			old, err := f.store.XNodePeriod(ctx, period.ID)
			if err != nil || old.UsedBytes != 12 {
				t.Fatal("actual traffic attribution", old, err)
			}
			compiled, err = f.service.Compile(ctx, f.node.ID)
			if err != nil {
				t.Fatal(err)
			}
			for _, user := range compiled.Users {
				if user.ID == int(probe.ProxyUserID) {
					t.Fatal("probe outlived quota or origin", action)
				}
			}
			if action == "reset" {
				current, err := f.store.XNodeIdentity(ctx, owner.ProxyUserID)
				if err != nil || current.CurrentPeriodID == period.ID {
					t.Fatal("reset did not create successor", err)
				}
				next, err := f.store.XNodePeriod(ctx, current.CurrentPeriodID)
				if err != nil || next.UsedBytes != 0 || next.Status != "open" {
					t.Fatal("late probe charged successor", next, err)
				}
				if old.Status != "closing" {
					t.Fatal("unfinished probe evidence lost", fmt.Sprint(old.Status))
				}
			}
		})
	}
}
