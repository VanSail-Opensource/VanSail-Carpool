package purchaseops

import (
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// QuoteMemberRefund evaluates local and provider-usage refund eligibility.
func QuoteMemberRefund(facts PurchaseFacts, userID string, usedBytes int64, now time.Time) (MemberRefundQuote, error) {
	purchase := facts.Purchase
	if purchase.UserID != userID {
		return MemberRefundQuote{}, ErrNotFound
	}
	expiresAt := purchase.CreatedAt.UTC().Add(refundWindow)
	quote := MemberRefundQuote{PurchaseID: purchase.ID, Refund: model.TXBMoney(purchase.PriceTXBMinor), QuotedAt: now.UTC(), EligibilityExpiresAt: &expiresAt}
	switch {
	case !activeAt(purchase, now):
		quote.ReasonCode = reason(ReasonNotActive)
	case !facts.FirstTerm:
		quote.ReasonCode = reason(ReasonNotFirstTerm)
	case now.Before(purchase.CreatedAt) || !now.Before(expiresAt):
		quote.ReasonCode = reason(ReasonWindowExpired)
	case usedBytes != 0:
		quote.ReasonCode = reason(ReasonTrafficUsed)
	default:
		quote.Eligible = true
	}
	return quote, nil
}

func activeAt(purchase model.Purchase, now time.Time) bool {
	return purchase.Status == "active" && !now.Before(purchase.ValidFrom) && now.Before(purchase.ValidUntil)
}
