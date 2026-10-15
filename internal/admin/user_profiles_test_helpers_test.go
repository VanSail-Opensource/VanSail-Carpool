package admin

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/affiliates"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

type userWorkflowRepositoryStub struct {
	user       model.User
	balance    model.Money
	purchases  []model.Purchase
	account    emby.Account
	accountErr error
	payments   []model.PaymentOrder
	refunds    []model.Refund
	operations []model.OperationReceipt
}

func (r *userWorkflowRepositoryStub) UserByID(context.Context, string) (model.User, error) {
	return r.user, nil
}

func (r *userWorkflowRepositoryStub) Balance(context.Context, string) (model.Money, error) {
	return r.balance, nil
}

func (r *userWorkflowRepositoryStub) ListPurchases(context.Context, string) ([]model.Purchase, error) {
	return append([]model.Purchase(nil), r.purchases...), nil
}

func (r *userWorkflowRepositoryStub) EmbyAccountForUser(context.Context, string) (emby.Account, error) {
	return r.account, r.accountErr
}

func (r *userWorkflowRepositoryStub) ListPaymentOrders(context.Context, string, int) ([]model.PaymentOrder, error) {
	return append([]model.PaymentOrder(nil), r.payments...), nil
}

func (r *userWorkflowRepositoryStub) ListRefundsForUser(context.Context, string, int) ([]model.Refund, error) {
	return append([]model.Refund(nil), r.refunds...), nil
}

func (r *userWorkflowRepositoryStub) ListAdminOperationsForUser(context.Context, string, int) ([]model.OperationReceipt, error) {
	return append([]model.OperationReceipt(nil), r.operations...), nil
}

func (*userWorkflowRepositoryStub) ListCouponGrants(context.Context, string, time.Time) ([]coupons.Grant, error) {
	return []coupons.Grant{}, nil
}

func (*userWorkflowRepositoryStub) SuccessfulAffiliateReferrals(context.Context, string, int) (affiliates.ReferralPage, error) {
	return affiliates.ReferralPage{Items: []affiliates.Referral{}}, nil
}

func (r *userWorkflowRepositoryStub) ComboByID(context.Context, string, bool) (model.Combo, error) {
	return model.Combo{}, nil
}

func (r *userWorkflowRepositoryStub) EditAdminEntitlement(context.Context, database.AdminEntitlementEditInput,
	time.Time) (model.Purchase, error) {
	return model.Purchase{}, nil
}

func (r *userWorkflowRepositoryStub) RefundAdminEntitlement(context.Context, database.AdminEntitlementRefundInput,
	time.Time) (model.OperationReceipt, error) {
	return model.OperationReceipt{}, nil
}

func (r *userWorkflowRepositoryStub) ReplaceAdminCombo(context.Context, database.AdminComboReplacementInput,
	time.Time) (model.OperationReceipt, error) {
	return model.OperationReceipt{}, nil
}

func (r *userWorkflowRepositoryStub) ResolveAdminOperation(context.Context, database.AdminOperationResolutionInput,
	time.Time) (model.OperationReceipt, error) {
	return model.OperationReceipt{}, nil
}

func (r *userWorkflowRepositoryStub) PreviewAdminBulkExtension(context.Context, database.AdminBulkExtensionFilter,
	time.Time) (database.AdminBulkExtensionPreview, error) {
	return database.AdminBulkExtensionPreview{}, nil
}

func (r *userWorkflowRepositoryStub) CreateAdminBulkExtension(context.Context, database.AdminBulkExtensionInput,
	time.Time) (model.OperationReceipt, error) {
	return model.OperationReceipt{}, nil
}

func (*userWorkflowRepositoryStub) GrantAdminCoupon(context.Context, database.AdminCouponGrantInput, time.Time) (coupons.Grant, error) {
	return coupons.Grant{}, nil
}

func (*userWorkflowRepositoryStub) DiscardAdminCoupon(context.Context, string, string, string, string, time.Time) error {
	return nil
}

func (*userWorkflowRepositoryStub) ActiveAdminTemporaryBan(context.Context, string) (*database.AdminTemporaryBan, error) {
	return nil, nil
}

func (*userWorkflowRepositoryStub) CreateAdminTemporaryBan(context.Context, database.AdminTemporaryBanInput, time.Time) (model.OperationReceipt, error) {
	return model.OperationReceipt{}, nil
}

func (*userWorkflowRepositoryStub) CreateAdminTemporaryUnban(context.Context, string, string, string, string, string, time.Time) (model.OperationReceipt, error) {
	return model.OperationReceipt{}, nil
}

func (*userWorkflowRepositoryStub) CreateAdminRemnaRelink(context.Context, database.AdminRemnaRelinkInput, time.Time) (model.OperationReceipt, error) {
	return model.OperationReceipt{}, nil
}
