package model

import "time"

// PurchaseAddonQuote is the non-mutating, server-priced preview for adding
// paid squads to the member's active purchase.
type PurchaseAddonQuote struct {
	RequiresQuoteVersion bool      `json:"-"`
	QuoteVersion         string    `json:"quoteVersion"`
	QuoteValidUntil      time.Time `json:"quoteValidUntil"`
	GrossPriceTXBMinor   int64     `json:"-"`
	DiscountTXBMinor     int64     `json:"-"`
	GrossPrice           Money     `json:"grossPrice"`
	Discount             Money     `json:"discount"`
	PurchaseID           string    `json:"purchaseId"`
	PriceTXBMinor        int64     `json:"-"`
	Price                Money     `json:"price"`
	ExpiresAt            time.Time `json:"expiresAt"`
	AddonSquadUUIDs      []string  `json:"addonSquadUuids"`
}
