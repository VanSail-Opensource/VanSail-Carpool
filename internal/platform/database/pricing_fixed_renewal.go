package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/X-Node/pkg/compute"
)

// Fixed decisions are completed synchronously from the existing authoritative
// renewal plan. Creating a purchase decision must never freeze legacy renewal
// prices by introducing a new settlement contract.
func fixedRenewalDecisionTx(ctx context.Context, tx *sql.Tx, user, purchase string, version int64, now time.Time) (*model.PricingDecision, error) {
	plan, err := automaticRenewalPlanTx(ctx, tx, user, purchase, nil, now)
	if err != nil {
		return nil, err
	}
	evidence, err := json.Marshal([]any{purchase, plan.Purchase.UpdatedAt, version, plan.GrossMinor, plan.DiscountMinor, plan.Addons})
	if err != nil {
		return nil, err
	}
	return &model.PricingDecision{Version: compute.Digest(evidence), Status: "UNCHANGED", Reason: "FIXED_PRICE", PublicPrice: model.TXBMoney(plan.GrossMinor), PersonalPrice: model.TXBMoney(plan.GrossMinor), NetPrice: model.TXBMoney(plan.NetMinor), Calibration: "NOT_APPLICABLE", GeneratedAt: now, ValidUntil: plan.ScheduledAt}, nil
}
