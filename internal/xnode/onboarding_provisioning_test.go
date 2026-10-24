package xnode_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestOnboardedMemberGetsStableSubscriptionWithoutPurchase(t *testing.T) {
	f := controlFixture(t, false)
	ctx := context.Background()
	user, _, err := f.store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: 99102, FirstName: "New member"}, false)
	if err != nil {
		t.Fatal(err)
	}
	job := model.OutboxJob{Kind: xnode.AccessRepairJob, Payload: `{"userId":"` + user.ID + `"}`}
	if err := f.service.RepairAccess(ctx, job); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.IdentityForUser(ctx, user.ID); !errors.Is(err, xnode.ErrNotFound) {
		t.Fatal("incomplete member provisioned", err)
	}
	if err := f.store.BeginAgreement(ctx, user.ID); err != nil {
		t.Fatal(err)
	}
	revision, ids, err := f.store.CurrentAgreementContract(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.CompleteOnboardingRevision(ctx, user.ID, revision, ids, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	var queued int
	if err := f.store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind=? AND json_extract(payload,'$.userId')=?`, job.Kind, user.ID).Scan(&queued); err != nil || queued != 1 {
		t.Fatal("onboarding did not queue credentials", queued, err)
	}
	var link string
	for range 2 {
		if err := f.service.RepairAccess(ctx, job); err != nil {
			t.Fatal(err)
		}
		metadata, err := f.service.OwnedSubscription(ctx, user.ID)
		if err != nil || metadata.URL == "" || metadata.DownloadURL == "" || metadata.Active {
			t.Fatal("subscription missing or unauthorized access granted", metadata, err)
		}
		if link != "" && link != metadata.URL {
			t.Fatal("retry rotated subscription")
		}
		link = metadata.URL
	}
	identity, err := f.service.IdentityForUser(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	periods, err := f.store.XNodePeriods(ctx, identity.ProxyUserID)
	if err != nil || len(periods) != 0 {
		t.Fatal("unpaid member received combo quota", periods, err)
	}
	compiled, err := f.service.Compile(ctx, f.node.ID)
	if err != nil || len(compiled.Users) != 0 {
		t.Fatal("credentials granted node admission", compiled.Users, err)
	}
}
