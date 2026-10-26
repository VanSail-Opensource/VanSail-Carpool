// Package renewal recommends renewal quotes; existing commerce owns settlement.
package renewal

import (
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

// Contract is an immutable system-calculated set of gross renewal terms.
type Contract struct {
	ID               string      `json:"id"`
	PurchaseID       string      `json:"purchaseId"`
	UserID           string      `json:"userId"`
	ProductID        string      `json:"productId"`
	Mode             string      `json:"mode"`
	Price            int64       `json:"contractualPriceMinor,string"`
	Ceiling          int64       `json:"priceLimitMinor,string"`
	MaxChangeBPS     int64       `json:"rateLimitBps"`
	PolicyVersion    int64       `json:"policyVersion"`
	RecordedAt       time.Time   `json:"recordedAt"`
	Version          int64       `json:"version"`
	SourcePurchaseID string      `json:"sourcePurchaseId"`
	Addons           []AddonTerm `json:"addonTerms"`
}

// AddonTerm retains the accepted unit basis when the live catalog later changes.
type AddonTerm struct {
	ID    string `json:"id"`
	Price int64  `json:"priceMinor,string"`
}

// Validate refuses invalid calculated bounds.
func (c Contract) Validate() error {
	if c.Mode != "DYNAMIC" && c.Mode != "FIXED" {
		return errors.New("invalid renewal contract mode")
	}
	if c.Price < 0 || c.Price > pricing.MaxTotalPrice || c.Ceiling < c.Price || c.Ceiling > pricing.MaxTotalPrice || c.MaxChangeBPS < 0 || c.MaxChangeBPS > 10000 {
		return pricing.RenewalQuoteUnavailable
	}
	total := int64(0)
	seen := map[string]bool{}
	for _, a := range c.Addons {
		if a.ID == "" || seen[a.ID] || a.Price < 0 || a.Price > c.Price-total {
			return pricing.RenewalQuoteUnavailable
		}
		total += a.Price
		seen[a.ID] = true
	}
	return nil
}
