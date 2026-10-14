package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/affiliates"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestAffiliateInviterFreezesUntilAuthentication(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	createTestUser(t, store, 801)
	createTestUser(t, store, 802)
	if _, accepted, err := store.AcceptAffiliateReferral(ctx, 803, 999, time.Now().UTC()); err != nil || accepted {
		t.Fatalf("missing inviter accepted/error = %v/%v", accepted, err)
	}
	if _, accepted, err := store.AcceptAffiliateReferral(ctx, 803, 801, time.Now().UTC()); err != nil || !accepted {
		t.Fatalf("valid inviter accepted/error = %v/%v", accepted, err)
	}
	if _, accepted, err := store.AcceptAffiliateReferral(ctx, 803, 802, time.Now().UTC()); err != nil || accepted {
		t.Fatalf("replacement inviter accepted/error = %v/%v", accepted, err)
	}
	user, _, err := store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: 803, LanguageCode: "zh-hans"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if user.NewUser || user.NotificationLocale != affiliates.LocaleChinese || user.InviterID == nil || *user.InviterID != 801 {
		t.Fatalf("authenticated referral user = %#v", user)
	}
	if _, accepted, err := store.AcceptAffiliateReferral(ctx, 803, 802, time.Now().UTC()); err != nil || accepted {
		t.Fatalf("authenticated replacement accepted/error = %v/%v", accepted, err)
	}
}

func TestAffiliateReferralRejectsSelf(t *testing.T) {
	store := newTestStore(t)
	createTestUser(t, store, 901)
	_, _, err := store.AcceptAffiliateReferral(context.Background(), 901, 901, time.Now().UTC())
	if !errors.Is(err, affiliates.ErrInvalidInput) {
		t.Fatalf("error = %v, want invalid input", err)
	}
}

func TestAffiliateReferralsUseTelegramFullName(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	inviter := createTestUser(t, store, 902)
	if _, accepted, err := store.AcceptAffiliateReferral(ctx, 903, 902, time.Now().UTC()); err != nil || !accepted {
		t.Fatalf("referral accepted/error = %v/%v", accepted, err)
	}
	if _, _, err := store.UpsertTelegramUser(ctx, model.TelegramProfile{
		ID: 903, FirstName: "Ada", LastName: "Lovelace", Username: "unused_username",
	}, false); err != nil {
		t.Fatalf("UpsertTelegramUser(): %v", err)
	}

	page, err := store.AffiliateReferrals(ctx, inviter.ID, 1)
	if err != nil {
		t.Fatalf("AffiliateReferrals(): %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].FirstName != "Ada" || page.Items[0].LastName != "Lovelace" {
		t.Fatalf("referral page = %#v", page)
	}
}
