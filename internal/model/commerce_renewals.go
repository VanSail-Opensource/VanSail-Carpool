package model

import "time"

// RenewalQuote is the retained server-priced preview for a legacy renewal batch.
type RenewalQuote struct {
	PurchaseID      string      `json:"purchaseId"`
	ComboID         string      `json:"comboId"`
	TermCount       int         `json:"termCount"`
	GrossPrice      Money       `json:"grossPrice"`
	Discount        Money       `json:"discount"`
	PricePerTerm    Money       `json:"pricePerTerm"`
	TotalPrice      Money       `json:"totalPrice"`
	CouponGrantID   *string     `json:"couponGrantId"`
	EffectiveAt     time.Time   `json:"effectiveAt"`
	ExpiresAt       time.Time   `json:"expiresAt"`
	AddonSquadUUIDs []string    `json:"addonSquadUuids"`
	AccessibleNodes []ProxyNode `json:"accessibleNodes"`
}

// RenewalBatch records one retained legacy renewal debit and its generated terms.
type RenewalBatch struct {
	ID         string     `json:"id"`
	PurchaseID string     `json:"purchaseId"`
	TermCount  int        `json:"termCount"`
	TotalPrice Money      `json:"totalPrice"`
	Purchases  []Purchase `json:"purchases"`
}
