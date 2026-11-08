package model

import "time"

const (
	HourlyPreparing        = "preparing"
	HourlyActive           = "active"
	HourlyFundingSuspended = "funding_suspended"
	HourlyDisabled         = "disabled"
	HourlyClosed           = "closed"
)

// HourlyEnrollment owns one principal/period for one explicit activation.
type HourlyEnrollment struct {
	ID                string     `json:"id"`
	OwnerID           string     `json:"-"`
	AccountGeneration int64      `json:"-"`
	GroupID           string     `json:"groupId"`
	ActivationKey     string     `json:"-"`
	Fingerprint       string     `json:"-"`
	Status            string     `json:"status"`
	ActivatedAt       *time.Time `json:"activatedAt"`
	PaidThrough       *time.Time `json:"paidThrough"`
	DisabledAt        *time.Time `json:"disabledAt"`
	CooldownUntil     *time.Time `json:"cooldownUntil"`
	ProxyUserID       int64      `json:"-"`
	PeriodID          string     `json:"-"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

// HourlySettlement is an immutable receipt, unique per enrollment and UTC hour.
type HourlySettlement struct {
	ID                  string    `json:"id"`
	EnrollmentID        string    `json:"enrollmentId"`
	HourStart           time.Time `json:"hourStart"`
	ChargedFrom         time.Time `json:"chargedFrom"`
	PaidUntil           time.Time `json:"paidUntil"`
	HourlyPriceTXBMinor int64     `json:"-"`
	ChargedTXBMinor     int64     `json:"-"`
	HourlyPrice         Money     `json:"hourlyPrice"`
	Charged             Money     `json:"charged"`
	OfferRevision       int64     `json:"offerRevision,string"`
	PolicyVersion       string    `json:"-"`
	LedgerID            string    `json:"-"`
	CreatedAt           time.Time `json:"createdAt"`
}

// HourlyExtraPreference authorizes future blocks with an explicitly accepted cap.
type HourlyExtraPreference struct {
	OwnerID                    string    `json:"-"`
	AccountGeneration          int64     `json:"-"`
	ExtraEnabled               bool      `json:"extraEnabled"`
	AcceptedExtraPriceTXBMinor *int64    `json:"acceptedExtraPriceTXBMinor,omitempty,string"`
	UpdatedAt                  time.Time `json:"updatedAt"`
}

// HourlyExtraPurchase joins one wallet debit to one decimal GB in its UTC day.
type HourlyExtraPurchase struct {
	ID                string    `json:"id"`
	OwnerID           string    `json:"-"`
	AccountGeneration int64     `json:"-"`
	DayUTC            string    `json:"dayUTC"`
	BlockIndex        int64     `json:"blockIndex,string"`
	ByteCount         int64     `json:"byteCount,string"`
	UnitPriceTXBMinor int64     `json:"-"`
	UnitPrice         Money     `json:"unitPrice"`
	PolicyVersion     string    `json:"-"`
	LedgerID          string    `json:"-"`
	CreatedAt         time.Time `json:"createdAt"`
}
