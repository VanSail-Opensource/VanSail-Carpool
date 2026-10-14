package admin

import (
	"context"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/rollover"
	"strconv"
	"time"
)

// ErrRefundUsageUnavailable prevents a fabricated refund suggestion on outages.
var ErrRefundUsageUnavailable = errors.New("refund traffic usage is unavailable")

type refundUsageSource interface {
	UsageSnapshotForRollover(context.Context, string, time.Time, time.Time) (rollover.UsageSnapshot, error)
}

// EntitlementRefundQuote is an editable suggestion, not a settlement command.
type EntitlementRefundQuote struct {
	PurchaseID            string       `json:"purchaseId"`
	Paid                  model.Money  `json:"paid"`
	AllocatedTrafficBytes *string      `json:"allocatedTrafficBytes"`
	UsedTrafficBytes      *string      `json:"usedTrafficBytes"`
	SuggestedRefund       *model.Money `json:"suggestedRefund"`
	QuotedAt              time.Time    `json:"quotedAt"`
	ReasonCode            *string      `json:"reasonCode"`
}

// RefundEntitlementQuote reuses strict queued usage reads without quiescing access.
func (s *UserWorkflows) RefundEntitlementQuote(ctx context.Context, userID, purchaseID string) (EntitlementRefundQuote, error) {
	user, err := s.repository.UserByID(ctx, userID)
	if err != nil {
		return EntitlementRefundQuote{}, err
	}
	purchases, err := s.repository.ListPurchases(ctx, userID)
	if err != nil {
		return EntitlementRefundQuote{}, err
	}
	var purchase *model.Purchase
	for i := range purchases {
		if purchases[i].ID == purchaseID && purchases[i].UserID == userID {
			purchase = &purchases[i]
			break
		}
	}
	if purchase == nil {
		return EntitlementRefundQuote{}, database.ErrNotFound
	}
	p := *purchase
	if p.Status != "active" && p.Status != "activating" && p.Status != "queued" {
		return EntitlementRefundQuote{}, database.ErrConflict
	}
	now := s.now().UTC()
	quote := EntitlementRefundQuote{PurchaseID: p.ID, Paid: model.TXBMoney(p.PriceTXBMinor), QuotedAt: now}
	if p.TrafficLimitBytes <= 0 || !p.ValidUntil.After(p.ValidFrom) || p.PriceTXBMinor < 0 || !validResetStrategy(p.ResetStrategy) {
		return unavailableRefundQuote(quote, "REFUND_ALLOCATION_UNAVAILABLE"), nil
	}
	snapshot := rollover.UsageSnapshot{LimitBytes: p.TrafficLimitBytes, Strategy: p.ResetStrategy}
	if p.Status != "queued" && !now.Before(p.ValidFrom) {
		if user.RemnaUserID == nil || s.refundUsage == nil {
			return unavailableRefundQuote(quote, "REFUND_USAGE_UNAVAILABLE"), nil
		}
		asOf := now
		if asOf.After(p.ValidUntil) {
			asOf = p.ValidUntil
		}
		snapshot, err = s.refundUsage.UsageSnapshotForRollover(ctx, *user.RemnaUserID, p.ValidFrom, asOf)
		if err != nil {
			return quote, fmt.Errorf("%w: %w", ErrRefundUsageUnavailable, err)
		}
		if !snapshot.NodeSeriesAvailable || snapshot.WeightedUsedBytes < 0 {
			return unavailableRefundQuote(quote, "REFUND_USAGE_UNAVAILABLE"), nil
		}
		// Purchased overrides define the allowance, not the mutable remote limit.
		snapshot.LimitBytes, snapshot.Strategy = p.TrafficLimitBytes, p.ResetStrategy
		if snapshot.LastResetAt != nil && snapshot.LastResetAt.After(asOf) {
			snapshot.LastResetAt = nil
		}
	}
	allocated := rollover.CalculateUsage(p, 0, snapshot).AllocatedBytes
	if allocated <= 0 {
		return unavailableRefundQuote(quote, "REFUND_ALLOCATION_UNAVAILABLE"), nil
	}
	used := snapshot.WeightedUsedBytes
	remaining := int64(0)
	if used < allocated {
		remaining = allocated - used
	}
	refund := model.TXBMoney(rollover.CreditMinor(p.PriceTXBMinor, remaining, allocated))
	allocatedText, usedText := strconv.FormatInt(allocated, 10), strconv.FormatInt(used, 10)
	quote.AllocatedTrafficBytes, quote.UsedTrafficBytes, quote.SuggestedRefund = &allocatedText, &usedText, &refund
	return quote, nil
}

func unavailableRefundQuote(quote EntitlementRefundQuote, reason string) EntitlementRefundQuote {
	quote.ReasonCode = &reason
	return quote
}
