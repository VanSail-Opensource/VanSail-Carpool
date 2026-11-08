package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
	"github.com/VanSail-Opensource/X-Node/pkg/compute"
	"time"
)

var ErrPricingQuoteChanged = errors.New("reviewed pricing quote changed")

type pricingQuoteComponent struct {
	Kind, ID string
	Decision *model.PricingDecision
}

func quoteEvidenceTx(ctx context.Context, tx *sql.Tx, user, coupon string, components []pricingQuoteComponent, now time.Time) ([]any, time.Time, bool, error) {
	until := now.UTC().Truncate(5 * time.Minute).Add(5 * time.Minute)
	evidence := []any{pricing.StrategyVersion, user}
	dynamic := false
	for _, c := range components {
		fixed, err := pricingReference(ctx, tx, c.Kind, c.ID)
		if err != nil {
			return nil, until, false, err
		}
		p, err := loadPricingPolicy(ctx, tx, c.Kind, c.ID, fixed)
		if err != nil {
			return nil, until, false, err
		}
		enabled := p.DynamicEnabled && p.PurchaseMode == "DYNAMIC"
		dynamic = dynamic || enabled
		if enabled && c.Decision != nil && (c.Decision.Status == "PENDING" || c.Decision.Status == "STALE" || c.Decision.Status == "FAILED") {
			return nil, until, dynamic, ErrPricingDecisionPending
		}
		if c.Decision != nil {
			if !c.Decision.ValidUntil.IsZero() && c.Decision.ValidUntil.Before(until) {
				until = c.Decision.ValidUntil
			}
			evidence = append(evidence, c.Decision.Version, c.Decision.CouponFingerprint)
		}
		evidence = append(evidence, p, fixed)
	}
	if coupon != "" {
		grant, err := grantByIDTx(ctx, tx, coupon)
		if err != nil {
			return nil, until, dynamic, err
		}
		evidence = append(evidence, grant)
		if grant.Coupon.ExpiresAt != nil && grant.Coupon.ExpiresAt.Before(until) {
			until = *grant.Coupon.ExpiresAt
		}
	}
	return evidence, until, dynamic, nil
}

func quoteDigest(evidence []any) (string, error) {
	raw, err := json.Marshal(evidence)
	if err != nil {
		return "", err
	}
	return compute.Digest(raw), nil
}

func bindPurchaseQuoteTx(ctx context.Context, tx *sql.Tx, input PurchaseInput, q *model.PurchaseQuote, addons []model.SquadProduct, now time.Time) (bool, error) {
	components := []pricingQuoteComponent{{Kind: "combo", ID: q.ComboID, Decision: q.PricingDecision}}
	for _, a := range addons {
		components = append(components, pricingQuoteComponent{Kind: "addon", ID: a.GroupID, Decision: a.PricingDecision})
	}
	evidence, until, dynamic, err := quoteEvidenceTx(ctx, tx, input.UserID, input.CouponGrantID, components, now)
	if err != nil {
		return false, err
	}
	effective := ""
	if q.Queued {
		effective = pricingStamp(q.EffectiveAt)
	}
	if q.RenewalDecision != nil {
		evidence = append(evidence, q.RenewalDecision.Version)
		dynamic = dynamic || q.RenewalDecision.DynamicEnabled
		if q.RenewalDecision.ValidUntil.Before(until) {
			until = q.RenewalDecision.ValidUntil
		}
	}
	evidence = append(evidence, input.ComboID, uniqueSorted(input.AddonSquadIDs), input.CouponGrantID, q.GrossPriceTXBMinor, q.DiscountTXBMinor, q.NetPriceTXBMinor, effective, pricingStamp(until))
	q.RequiresQuoteVersion = dynamic
	q.QuoteValidUntil = until
	q.QuoteVersion, err = quoteDigest(evidence)
	return dynamic, err
}

func bindAddonQuoteTx(ctx context.Context, tx *sql.Tx, input PurchaseAddonInput, q *model.PurchaseAddonQuote, addons []pricedPurchaseAddon, now time.Time) (bool, error) {
	components := []pricingQuoteComponent{}
	for _, a := range addons {
		components = append(components, pricingQuoteComponent{Kind: "addon", ID: a.product.GroupID, Decision: a.product.PricingDecision})
	}
	evidence, until, dynamic, err := quoteEvidenceTx(ctx, tx, input.UserID, input.CouponGrantID, components, now)
	if err != nil {
		return false, err
	}
	evidence = append(evidence, input.PurchaseID, uniqueSorted(input.AddonSquadIDs), input.CouponGrantID, q.GrossPriceTXBMinor, q.DiscountTXBMinor, q.PriceTXBMinor, pricingStamp(q.ExpiresAt), pricingStamp(until))
	q.RequiresQuoteVersion = dynamic
	q.QuoteValidUntil = until
	q.QuoteVersion, err = quoteDigest(evidence)
	return dynamic, err
}
func validateReviewedQuote(provided, current string, dynamic bool) error {
	if provided != "" && provided != current || dynamic && provided == "" {
		return ErrPricingQuoteChanged
	}
	return nil
}
