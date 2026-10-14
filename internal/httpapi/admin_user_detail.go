package httpapi

import (
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/abuse"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/affiliates"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connections"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

type adminEntitlementResponse struct {
	model.Purchase
	UserID string `json:"userId"`
}

type adminPaymentResponse struct {
	model.PaymentOrder
	UserID string `json:"userId"`
}

type adminSynchronizationResponse struct {
	Status       string  `json:"status"`
	RemoteUserID *string `json:"remoteUserId"`
	LastError    *string `json:"lastError"`
}

type adminUserDetailResponse struct {
	User                    userResponse                 `json:"user"`
	Balance                 model.Money                  `json:"balance"`
	Synchronization         adminSynchronizationResponse `json:"synchronization"`
	ActiveCombo             *adminEntitlementResponse    `json:"activeCombo"`
	Entitlements            []adminEntitlementResponse   `json:"entitlements"`
	EmbyAccounts            []embyAccountResponse        `json:"embyAccounts"`
	Payments                []adminPaymentResponse       `json:"payments"`
	Refunds                 []model.Refund               `json:"refunds"`
	Operations              []model.OperationReceipt     `json:"operations"`
	IPBlocks                []connections.IPBlock        `json:"ipBlocks"`
	CouponWallet            []coupons.Grant              `json:"couponWallet"`
	AbuseHistory            []abuse.Record               `json:"abuseHistory"`
	AffiliateHistory        affiliates.ReferralPage      `json:"affiliateHistory"`
	TemporaryBan            *database.AdminTemporaryBan  `json:"temporaryBan"`
	AutoTrafficResetEnabled bool                         `json:"autoTrafficResetEnabled"`
	PMConversation          *pmConversationResponse      `json:"pmConversation"`
}

func mapAdminUserDetail(detail admin.UserDetail) adminUserDetailResponse {
	response := adminUserDetailResponse{User: mapUser(detail.User), Balance: detail.Balance,
		Synchronization: adminSynchronizationResponse{Status: detail.Synchronization.Status,
			RemoteUserID: detail.Synchronization.RemoteUserID, LastError: detail.Synchronization.LastError},
		Entitlements: make([]adminEntitlementResponse, 0, len(detail.Entitlements)),
		EmbyAccounts: make([]embyAccountResponse, 0, len(detail.EmbyAccounts)),
		Payments:     make([]adminPaymentResponse, 0, len(detail.Payments)), Refunds: detail.Refunds,
		Operations: detail.Operations, IPBlocks: []connections.IPBlock{}, CouponWallet: detail.CouponWallet,
		AbuseHistory: detail.AbuseHistory, AffiliateHistory: detail.AffiliateHistory, TemporaryBan: detail.TemporaryBan,
		AutoTrafficResetEnabled: detail.User.AutoTrafficResetEnabled}
	for _, item := range detail.Entitlements {
		response.Entitlements = append(response.Entitlements, adminEntitlementResponse{Purchase: item, UserID: item.UserID})
	}
	if detail.ActiveCombo != nil {
		response.ActiveCombo = &adminEntitlementResponse{Purchase: *detail.ActiveCombo, UserID: detail.ActiveCombo.UserID}
	}
	for _, account := range detail.EmbyAccounts {
		response.EmbyAccounts = append(response.EmbyAccounts, mapEmbyAccount(account))
	}
	for _, payment := range detail.Payments {
		response.Payments = append(response.Payments, adminPaymentResponse{PaymentOrder: payment, UserID: payment.UserID})
	}
	return response
}
