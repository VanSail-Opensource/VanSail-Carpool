package purchaseops

import (
	"math"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// ResetPriceMinor calculates a cadence share and rounds up to one TXB minor unit.
func ResetPriceMinor(coreGrossMinor int64, strategy string) (int64, bool) {
	if coreGrossMinor <= 0 {
		return 0, false
	}
	divisor := int64(1)
	switch strategy {
	case "DAY":
		divisor = 30
	case "WEEK":
		divisor = 4
	case "MONTH_ROLLING":
	default:
		return 0, false
	}
	if coreGrossMinor > math.MaxInt64-(divisor-1) {
		return 0, false
	}
	return (coreGrossMinor + divisor - 1) / divisor, true
}

// QuoteTrafficReset evaluates local reset eligibility using immutable pricing.
func QuoteTrafficReset(facts PurchaseFacts, userID string, now time.Time) (TrafficResetQuote, error) {
	purchase := facts.Purchase
	if purchase.UserID != userID {
		return TrafficResetQuote{}, ErrNotFound
	}
	quote := TrafficResetQuote{PurchaseID: purchase.ID, ResetStrategy: purchase.ResetStrategy, QuotedAt: now.UTC(), Price: model.TXBMoney(0)}
	if !activeAt(purchase, now) {
		quote.ReasonCode = reason(ReasonNotActive)
		return quote, nil
	}
	price, valid := ResetPriceMinor(facts.CoreGrossMinor, purchase.ResetStrategy)
	if !valid {
		if facts.CoreGrossMinor <= 0 {
			quote.ReasonCode = reason(ReasonInvalidPrice)
		} else {
			quote.ReasonCode = reason(ReasonUnsupported)
		}
		return quote, nil
	}
	quote.Eligible = true
	quote.Price = model.TXBMoney(price)
	return quote, nil
}

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
