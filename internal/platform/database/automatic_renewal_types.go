package database

import (
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"time"
)

const (
	AutoRenewalReasonQueuedPurchase       = "QUEUED_PURCHASE"
	AutoRenewalReasonPurchaseUnavailable  = "PURCHASE_UNAVAILABLE"
	AutoRenewalReasonComboUnavailable     = "COMBO_UNAVAILABLE"
	AutoRenewalReasonPaidAddonUnavailable = "PAID_ADDON_UNAVAILABLE"
	AutoRenewalReasonDiscountUnavailable  = "RECURRING_DISCOUNT_UNAVAILABLE"
	AutoRenewalReasonInsufficientBalance  = "INSUFFICIENT_BALANCE"
	AutoRenewalReasonNoAccessibleNodes    = "NO_ACCESSIBLE_NODES"
	AutoRenewalReasonUnavailable          = "RENEWAL_UNAVAILABLE"
)

// AutoRenewalPlan is the local, current-price source for one next-cycle quote.
type AutoRenewalPlan struct {
	Purchase              model.Purchase
	Combo                 model.Combo
	Addons                []model.SquadProduct
	GrossMinor            int64
	DiscountMinor         int64
	NetMinor              int64
	ScheduledAt           time.Time
	NextCycleEndsAt       time.Time
	IneligibleReason      string
	trafficLimitOverride  *int64
	resetStrategyOverride *string
	squadUUIDsOverride    *string
	rewardRenewalPrice    *int64
	rewardRolloverBPS     *int64
	rewardTrafficRenewal  bool
}
