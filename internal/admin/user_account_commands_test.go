package admin

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func TestUserAccountCommandsForwardValidatedInputs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repository := &accountCommandRepository{}
	workflows := NewUserWorkflows(repository, nil)
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	workflows.now = func() time.Time { return now }

	if _, err := workflows.GrantCoupon(ctx, "admin", "member", " coupon ", "grant-key", " support adjustment "); err != nil {
		t.Fatal(err)
	}
	if repository.grant.CouponID != "coupon" || repository.grant.Reason != "support adjustment" || !repository.grantAt.Equal(now) {
		t.Fatalf("coupon grant = %+v at %s", repository.grant, repository.grantAt)
	}
	if err := workflows.DiscardCoupon(ctx, "admin", "member", "grant", "discard-key"); err != nil {
		t.Fatal(err)
	}
	if repository.discardGrantID != "grant" || repository.discardKey != "discard-key" {
		t.Fatalf("coupon discard = %q/%q", repository.discardGrantID, repository.discardKey)
	}
	if _, err := workflows.RefundEntitlement(ctx, "admin", "member", "purchase", "refund-key", "partial refund", 300); err != nil {
		t.Fatal(err)
	}
	if repository.refund.AmountTXBMinor != 300 || repository.refund.Reason != "partial refund" || len(repository.refund.RequestFingerprint) != 64 {
		t.Fatalf("refund = %+v", repository.refund)
	}
	if _, err := workflows.TemporaryBan(ctx, "admin", "member", "ban-key", "investigation", 60); err != nil {
		t.Fatal(err)
	}
	if repository.ban.DurationMinutes != 60 || len(repository.ban.RequestFingerprint) != 64 {
		t.Fatalf("temporary ban = %+v", repository.ban)
	}
	if _, err := workflows.TemporaryUnban(ctx, "admin", "member", "unban-key", "cleared"); err != nil {
		t.Fatal(err)
	}
	if repository.unban.UserID != "member" || repository.unban.Reason != "cleared" || len(repository.unban.Fingerprint) != 64 {
		t.Fatalf("temporary unban = %+v", repository.unban)
	}
	if _, err := workflows.TemporaryBan(ctx, "admin", "member", "bad-ban", "investigation", 0); err == nil {
		t.Fatal("TemporaryBan() accepted zero duration")
	}
}

type accountCommandRepository struct {
	userWorkflowRepositoryStub
	grant          database.AdminCouponGrantInput
	grantAt        time.Time
	discardGrantID string
	discardKey     string
	refund         database.AdminEntitlementRefundInput
	ban            database.AdminTemporaryBanInput
	unban          struct{ UserID, Reason, Fingerprint string }
}

func (r *accountCommandRepository) GrantAdminCoupon(_ context.Context, input database.AdminCouponGrantInput, now time.Time) (coupons.Grant, error) {
	r.grant, r.grantAt = input, now
	return coupons.Grant{ID: "grant"}, nil
}

func (r *accountCommandRepository) DiscardAdminCoupon(_ context.Context, _ string, _ string, grantID, key string, _ time.Time) error {
	r.discardGrantID, r.discardKey = grantID, key
	return nil
}

func (r *accountCommandRepository) RefundAdminEntitlement(_ context.Context, input database.AdminEntitlementRefundInput, _ time.Time) (model.OperationReceipt, error) {
	r.refund = input
	return model.OperationReceipt{ID: "refund"}, nil
}

func (r *accountCommandRepository) CreateAdminTemporaryBan(_ context.Context, input database.AdminTemporaryBanInput, _ time.Time) (model.OperationReceipt, error) {
	r.ban = input
	return model.OperationReceipt{ID: "ban"}, nil
}

func (r *accountCommandRepository) CreateAdminTemporaryUnban(_ context.Context, _ string, userID, _ string, fingerprint, reason string, _ time.Time) (model.OperationReceipt, error) {
	r.unban = struct{ UserID, Reason, Fingerprint string }{userID, reason, fingerprint}
	return model.OperationReceipt{ID: "unban"}, nil
}
