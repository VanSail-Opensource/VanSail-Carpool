package model

import (
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/squadprofile"
)

// Money is a currency amount encoded without JSON floating point.
type Money struct {
	Currency string `json:"currency"`
	Minor    string `json:"minor"`
	Display  string `json:"display"`
}

// SquadProduct enriches an upstream Remnawave internal squad with customer-facing catalog data.
type SquadProduct struct {
	ID                     string                `json:"id"`
	RemnaSquadUUID         string                `json:"remnaSquadUuid"`
	Name                   string                `json:"name"`
	Description            string                `json:"description"`
	Profile                *squadprofile.Profile `json:"profile"`
	PriceTXBMinor          int64                 `json:"-"`
	Price                  Money                 `json:"price"`
	Visible                bool                  `json:"visible"`
	UpstreamPresent        bool                  `json:"upstreamPresent"`
	StockLimit             *int                  `json:"stockLimit"`
	StockRemaining         *int                  `json:"stockRemaining"`
	StockHeldByCurrentUser bool                  `json:"stockHeldByCurrentUser"`
	ActivationRequired     bool                  `json:"activationRequired"`
	AccessibleNodes        []CatalogNode         `json:"accessibleNodes"`
	CreatedAt              time.Time             `json:"createdAt"`
	UpdatedAt              time.Time             `json:"updatedAt"`
}

// Combo is a time-limited traffic entitlement and its included squads.
type Combo struct {
	IPLookupQuota           *int           `json:"ipLookupQuota"`
	ID                      string         `json:"id"`
	Name                    string         `json:"name"`
	Description             string         `json:"description"`
	PriceTXBMinor           int64          `json:"-"`
	Price                   Money          `json:"price"`
	ValidityDays            int            `json:"validityDays"`
	TrafficLimitBytes       int64          `json:"-"`
	TrafficLimit            string         `json:"trafficLimitBytes"`
	ResetStrategy           string         `json:"resetStrategy"`
	Active                  bool           `json:"active"`
	IncludedSquads          []SquadProduct `json:"includedSquads"`
	RolloverMinRemainingBPS int            `json:"rolloverMinRemainingBps"`
	CreatedAt               time.Time      `json:"createdAt"`
	UpdatedAt               time.Time      `json:"updatedAt"`
}

// Purchase represents an active, queued, or historical combo term.
type Purchase struct {
	ID                        string    `json:"id"`
	UserID                    string    `json:"-"`
	ComboID                   string    `json:"comboId"`
	ComboName                 string    `json:"comboName"`
	PriceTXBMinor             int64     `json:"-"`
	Price                     Money     `json:"price"`
	GrossPriceTXBMinor        int64     `json:"-"`
	GrossPrice                Money     `json:"grossPrice"`
	CoreGrossTXBMinor         int64     `json:"-"`
	CouponDiscountTXBMinor    int64     `json:"-"`
	CouponDiscount            Money     `json:"couponDiscount"`
	CouponGrantID             *string   `json:"couponGrantId"`
	ValidFrom                 time.Time `json:"validFrom"`
	ValidUntil                time.Time `json:"validUntil"`
	Status                    string    `json:"status"`
	AutoRenewEnabled          bool      `json:"autoRenewEnabled"`
	RecurringDiscountAttached bool      `json:"-"`
	TrafficLimitBytes         int64     `json:"-"`
	TrafficLimit              string    `json:"trafficLimitBytes"`
	ResetStrategy             string    `json:"resetStrategy"`
	SquadUUIDs                []string  `json:"squadUuids"`
	RolloverMinRemainingBPS   int       `json:"rolloverMinRemainingBps"`
	CreatedAt                 time.Time `json:"createdAt"`
	UpdatedAt                 time.Time `json:"updatedAt"`
}

// PurchaseRollover is the durable expiry settlement for one paid term.
type PurchaseRollover struct {
	PurchaseID          string     `json:"purchaseId"`
	Status              string     `json:"status"`
	TrafficLimitBytes   int64      `json:"trafficLimitBytes"`
	AllocatedBytes      *int64     `json:"allocatedTrafficBytes"`
	UsedTrafficBytes    *int64     `json:"usedTrafficBytes"`
	EligibleUnusedBytes *int64     `json:"eligibleUnusedBytes"`
	RemainingBytes      *int64     `json:"remainingTrafficBytes"`
	MinimumRemainingBPS int        `json:"minimumRemainingBps"`
	NetPaidTXBMinor     int64      `json:"-"`
	CreditedTXBMinor    int64      `json:"-"`
	ExceptionCode       string     `json:"exceptionCode"`
	Attempts            int        `json:"attempts"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
	CompletedAt         *time.Time `json:"completedAt"`
	AlgorithmVersion    string     `json:"algorithmVersion"`
}

// RolloverUsageSummary is the bounded settlement aggregate retained in SQLite.
// AllocatedBytes and UsedBytes cover every included cadence interval, while
// EligibleUnusedBytes includes only intervals strictly above the threshold.
type RolloverUsageSummary struct {
	AllocatedBytes      int64
	UsedBytes           int64
	EligibleUnusedBytes int64
	AlgorithmVersion    string
}

// RolloverProjection is a live, aggregate-only view of the current term's
// possible rollover credit. It is never persisted and never contains raw
// provider usage series.
type RolloverProjection struct {
	PurchaseID                  string          `json:"purchaseId"`
	Paid                        Money           `json:"paid"`
	AutoRenewalEnabled          bool            `json:"autoRenewalEnabled"`
	WarningCode                 *string         `json:"warningCode"`
	ActualUsedTrafficBytes      *int64          `json:"actualUsedTrafficBytes,string"`
	ProjectedFullTermUsageBytes *int64          `json:"projectedFullTermUsageBytes,string"`
	MaximumAllowableUsageBytes  *int64          `json:"maximumAllowableUsageBytes,string"`
	MaximumDailyUsageBytes      *int64          `json:"maximumDailyUsageBytes,string"`
	PredictedRollover           *Money          `json:"predictedRollover"`
	RequiredReductionBytes      *int64          `json:"requiredReductionBytes,string"`
	RequiredDailyReductionBytes *int64          `json:"requiredDailyReductionBytes,string"`
	MinimumRemainingBPS         int             `json:"minimumRemainingBps"`
	Term                        *RolloverWindow `json:"term"`
	LastResetPeriod             *RolloverWindow `json:"lastResetPeriod"`
	FetchedAt                   time.Time       `json:"fetchedAt"`
}

// RolloverWindow describes one aggregate cadence window used by a projection.
type RolloverWindow struct {
	Start                 time.Time `json:"start"`
	End                   time.Time `json:"end"`
	AllocatedTrafficBytes int64     `json:"allocatedTrafficBytes,string"`
	UsedTrafficBytes      int64     `json:"usedTrafficBytes,string"`
	RemainingTrafficBytes int64     `json:"remainingTrafficBytes,string"`
	EligibleUnusedBytes   int64     `json:"eligibleUnusedBytes,string"`
	Rollover              Money     `json:"rollover"`
}

// CatalogNode is the display-only node identity and multiplier metadata used
// by the member dashboard.
type CatalogNode struct {
	UUID                  string   `json:"uuid"`
	Name                  string   `json:"name"`
	CountryCode           string   `json:"countryCode"`
	ConsumptionMultiplier float64  `json:"consumptionMultiplier"`
	ProviderName          *string  `json:"providerName"`
	ActiveInboundUUIDs    []string `json:"-"`
	ProviderFaviconURL    *string  `json:"-"`
}

// Catalog is the complete customer-visible catalog snapshot.
type Catalog struct {
	Combos []Combo        `json:"combos"`
	Addons []SquadProduct `json:"addons"`
	Nodes  []CatalogNode  `json:"nodes"`
}

// PurchaseQuote is a server-priced, non-mutating checkout preview.
type PurchaseQuote struct {
	ComboID            string      `json:"comboId"`
	ComboName          string      `json:"comboName"`
	GrossPriceTXBMinor int64       `json:"-"`
	DiscountTXBMinor   int64       `json:"-"`
	NetPriceTXBMinor   int64       `json:"-"`
	GrossPrice         Money       `json:"grossPrice"`
	Discount           Money       `json:"discount"`
	NetPrice           Money       `json:"netPrice"`
	EffectiveAt        time.Time   `json:"effectiveAt"`
	ExpiresAt          time.Time   `json:"expiresAt"`
	Queued             bool        `json:"queued"`
	AddonSquadUUIDs    []string    `json:"addonSquadUuids"`
	AccessibleNodes    []RemnaNode `json:"accessibleNodes"`
}
