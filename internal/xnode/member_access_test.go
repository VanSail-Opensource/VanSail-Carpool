package xnode_test

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestPaidMemberWithoutPanelUsernameHasHostsAndAdmission(t *testing.T) {
	f := controlFixture(t, true)
	ctx := context.Background()
	if _, err := f.store.DB().ExecContext(ctx, `UPDATE users SET username=NULL WHERE id=?`, f.user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.SaveEndpoint(ctx, xnode.Endpoint{NodeID: f.node.ID, InboundTag: "paid", Name: "Paid host", Address: "proxy.example", Port: 443, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	config, err := f.service.Compile(ctx, f.node.ID)
	if err != nil || len(config.Users) != 1 {
		t.Fatalf("paid admission: %v, %+v", err, config.Users)
	}
	metadata, err := f.service.OwnedSubscription(ctx, f.user.ID)
	if err != nil || !metadata.Active || metadata.Incomplete {
		t.Fatalf("subscription: %v, %+v", err, metadata)
	}
}

func TestMissingPeriodPointerIsPendingAndRecoveryRetainsUsage(t *testing.T) {
	f := controlFixture(t, true)
	ctx := context.Background()
	identity, err := f.store.XNodeIdentityByUser(ctx, f.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.store.DB().ExecContext(ctx, `UPDATE xnode_periods SET used_bytes=12345 WHERE id=?`, identity.CurrentPeriodID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.store.DB().ExecContext(ctx, `UPDATE xnode_identities SET current_period_id=NULL WHERE user_id=?`, f.user.ID); err != nil {
		t.Fatal(err)
	}
	link, err := f.service.SubscriptionURL(identity)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = f.service.BearerSubscription(ctx, strings.TrimPrefix(u.Path, "/subscription/"), "native")
	var policy *xnode.PolicyError
	if !errors.As(err, &policy) || policy.Code != "XNODE_ACCESS_PENDING" {
		t.Fatalf("pending became 404: %v", err)
	}
	job := model.OutboxJob{Kind: xnode.AccessRepairJob, Payload: `{"userId":"` + f.user.ID + `"}`}
	if err = f.service.RepairAccess(ctx, job); err != nil {
		t.Fatal(err)
	}
	after, err := f.store.XNodeIdentityByUser(ctx, f.user.ID)
	if err != nil || after.CurrentPeriodID != identity.CurrentPeriodID {
		t.Fatalf("replaced period: %+v %v", after, err)
	}
	period, err := f.store.XNodePeriod(ctx, after.CurrentPeriodID)
	if err != nil || period.UsedBytes != 12345 {
		t.Fatalf("lost usage: %+v %v", period, err)
	}
	if err = f.service.RepairAccess(ctx, job); err != nil {
		t.Fatal("idempotent recovery", err)
	}
}

func TestRecoveryDoesNotResetClosedOrExhaustedPurchase(t *testing.T) {
	for _, state := range []string{"closed", "exhausted"} {
		t.Run(state, func(t *testing.T) {
			f := controlFixture(t, true)
			ctx := context.Background()
			identity, err := f.store.XNodeIdentityByUser(ctx, f.user.ID)
			if err != nil {
				t.Fatal(err)
			}
			query := `UPDATE xnode_periods SET status='closed' WHERE id=?`
			if state == "exhausted" {
				query = `UPDATE xnode_periods SET used_bytes=limit_bytes WHERE id=?`
			}
			if _, err = f.store.DB().ExecContext(ctx, query, identity.CurrentPeriodID); err != nil {
				t.Fatal(err)
			}
			if err = f.store.EnqueueXNodeAccessRepairs(ctx, time.Now().UTC()); err != nil {
				t.Fatal(err)
			}
			err = f.service.RepairAccess(ctx, model.OutboxJob{Payload: `{"userId":"` + f.user.ID + `"}`})
			if state == "closed" && !errors.Is(err, xnode.ErrAccessAccountingPending) {
				t.Fatalf("settled repair lost its accounting reason: %v", err)
			}
			periods, err := f.store.XNodePeriods(ctx, identity.ProxyUserID)
			if err != nil || len(periods) != 1 {
				t.Fatalf("new allowance created: %+v %v", periods, err)
			}
			if state == "closed" && periods[0].Status != "closed" {
				t.Fatal("settled period reopened")
			}
			config, err := f.service.Compile(ctx, f.node.ID)
			if err != nil || len(config.Users) != 0 {
				t.Fatalf("unauthorized admission: %+v %v", config.Users, err)
			}
		})
	}
}
