package model

import "github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"

type UserProductPrice struct {
	ProductType     string           `json:"productType"`
	ProductID       string           `json:"productId"`
	Name            string           `json:"name"`
	BillingMode     string           `json:"billingMode"`
	FixedPrice      Money            `json:"fixedPrice"`
	PublicPrice     Money            `json:"publicPrice"`
	PersonalPrice   Money            `json:"personalPrice"`
	NetPrice        Money            `json:"netPrice"`
	CampaignPrice   Money            `json:"campaignPrice"`
	Coupons         []coupons.Grant  `json:"coupons"`
	Decision        PricingDecision  `json:"decision"`
	RenewalDecision *PricingDecision `json:"renewalDecision"`
}

type UserPricePage struct {
	Items      []UserProductPrice `json:"items"`
	NextCursor string             `json:"nextCursor"`
}
