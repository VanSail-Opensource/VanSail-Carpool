package model

import "time"

// PricingDecision is a versioned member offer, independent of wallet funding.
type PricingDecision struct {
	CouponFingerprint string    `json:"-"`
	DynamicEnabled    bool      `json:"dynamicEnabled"`
	Version           string    `json:"version"`
	Status            string    `json:"status"`
	Reason            string    `json:"reason"`
	PublicPrice       Money     `json:"publicPrice"`
	PersonalPrice     Money     `json:"personalPrice"`
	NetPrice          Money     `json:"netPrice"`
	BasketGrossPrice  *Money    `json:"basketGrossPrice,omitempty"`
	Probability       *float64  `json:"probability,omitempty"`
	ModelVersion      string    `json:"modelVersion"`
	Calibration       string    `json:"calibration"`
	GeneratedAt       time.Time `json:"generatedAt"`
	ValidUntil        time.Time `json:"validUntil"`
}
