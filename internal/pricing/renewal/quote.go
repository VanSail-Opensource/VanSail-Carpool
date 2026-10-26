package renewal

import (
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

// Quote holds immutable price/authorization evidence; only lifecycle fields advance.
type Quote struct {
	ID                string     `json:"id"`
	PurchaseID        string     `json:"purchaseId"`
	ContractID        string     `json:"contractId"`
	Price             int64      `json:"priceMinor,string"`
	PolicyVersion     int64      `json:"policyVersion"`
	ContractVersion   int64      `json:"contractVersion"`
	State             string     `json:"state"`
	GeneratedAt       time.Time  `json:"generatedAt"`
	AuthorizedAt      *time.Time `json:"authorizedAt"`
	SettleAfter       time.Time  `json:"settleAfter"`
	ValidUntil        time.Time  `json:"validUntil"`
	SettledPurchaseID *string    `json:"settledPurchaseId"`
	Probability       float64    `json:"probability"`
	Contribution      float64    `json:"contribution"`
	HealthState       string     `json:"healthState"`
	ModelVersion      string     `json:"modelVersion"`
	Trace             string     `json:"trace"`
}

// Settleable revalidates the immutable settlement window without recalculating quoted prices.
func (q Quote) Settleable(c Contract, now time.Time) error {
	if q.ContractID != c.ID || q.ContractVersion != c.Version || q.Price > c.Ceiling || q.Price > c.Price+c.Price*c.MaxChangeBPS/10000 || q.AuthorizedAt == nil || now.Before(q.SettleAfter) || !now.Before(q.ValidUntil) {
		return pricing.RenewalQuoteUnavailable
	}
	if q.State != "AUTHORIZED" && q.State != "READY_TO_SETTLE" && q.State != "DEFERRED_HEALTH" {
		return pricing.RenewalQuoteUnavailable
	}
	return nil
}
