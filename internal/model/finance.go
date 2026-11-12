package model

import (
	"fmt"
	"strconv"
	"time"
)

// MinorInt64 returns the exact integer representation used by internal atomic debits.
func (money Money) MinorInt64() int64 {
	value, _ := strconv.ParseInt(money.Minor, 10, 64)
	return value
}

// LedgerEntry is an immutable TXB balance mutation.
type LedgerEntry struct {
	ID              string    `json:"id"`
	DeltaTXBMinor   int64     `json:"-"`
	Delta           Money     `json:"delta"`
	BalanceAfterRaw int64     `json:"-"`
	BalanceAfter    Money     `json:"balanceAfter"`
	Kind            string    `json:"kind"`
	ReferenceID     string    `json:"referenceId"`
	Note            string    `json:"note"`
	CreatedAt       time.Time `json:"createdAt"`
}

// PaymentOrder is a provider checkout attempt.
type PaymentOrder struct {
	ID                   string     `json:"id"`
	UserID               string     `json:"-"`
	Provider             string     `json:"provider"`
	MethodID             string     `json:"methodId"`
	ProviderRail         string     `json:"providerRail"`
	Status               string     `json:"status"`
	TXBMinor             int64      `json:"-"`
	TXB                  Money      `json:"txb"`
	PayableAmount        string     `json:"payableAmount"`
	PayableCurrency      string     `json:"payableCurrency"`
	RateSnapshot         string     `json:"rateSnapshot"`
	RateDirection        string     `json:"rateDirection"`
	ProviderTradeID      *string    `json:"-"`
	ProviderChargeID     *string    `json:"-"`
	PaymentURL           *string    `json:"paymentUrl"`
	QRPayload            *string    `json:"qrPayload"`
	ReceivingAddress     *string    `json:"receivingAddress"`
	ActualCryptoAmount   *string    `json:"actualCryptoAmount"`
	ActualCryptoCurrency *string    `json:"actualCryptoCurrency"`
	ExpiresAt            time.Time  `json:"expiresAt"`
	PaidAt               *time.Time `json:"paidAt"`
	RefundedAt           *time.Time `json:"refundedAt"`
	CancelledAt          *time.Time `json:"cancelledAt"`
	CancelReason         string     `json:"cancelReason"`
	ProviderCancelStatus string     `json:"providerCancelStatus"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
}

// PaymentReturnDetails is the deliberately narrow receipt projection exposed
// through a short-lived, signed provider-return capability.
type PaymentReturnDetails struct {
	ID                   string     `json:"id"`
	Provider             string     `json:"provider"`
	ProviderRail         string     `json:"providerRail"`
	Status               string     `json:"status"`
	TXB                  Money      `json:"txb"`
	PayableAmount        string     `json:"payableAmount"`
	PayableCurrency      string     `json:"payableCurrency"`
	ActualCryptoAmount   *string    `json:"actualCryptoAmount"`
	ActualCryptoCurrency *string    `json:"actualCryptoCurrency"`
	CreatedAt            time.Time  `json:"createdAt"`
	PaidAt               *time.Time `json:"paidAt"`
}

// PaymentMethod is one selectable provider rail exposed to a member.
type PaymentMethod struct {
	ID             string `json:"id"`
	Provider       string `json:"provider"`
	ProfileID      string `json:"profileId"`
	ProviderName   string `json:"providerName"`
	Rail           string `json:"rail"`
	Name           string `json:"name"`
	Currency       string `json:"currency"`
	CryptoCurrency string `json:"cryptoCurrency"`
	Network        string `json:"network"`
	NetworkName    string `json:"networkName"`
	Available      bool   `json:"available"`
	Note           string `json:"note"`
	Mode           string `json:"mode"`
}

// PaymentChannel is one provider-discovered crypto currency and network pair.
type PaymentChannel struct {
	Rail        string
	Currency    string
	Network     string
	NetworkName string
}

// PaymentProfile is the masked administrative representation of one provider account.
type PaymentProfile struct {
	ID              string    `json:"id"`
	Provider        string    `json:"provider"`
	ProviderName    string    `json:"providerName"`
	EnabledChannels []string  `json:"enabledChannels"`
	Endpoint        string    `json:"endpoint"`
	MerchantID      string    `json:"merchantId"`
	Credential      string    `json:"credential"`
	Acknowledgement string    `json:"acknowledgement"`
	Enabled         bool      `json:"enabled"`
	Configured      bool      `json:"configured"`
	Rail            string    `json:"-"`
	UpdatedAt       time.Time `json:"-"`
}

// PaymentProfileRuntime is an internal-only profile with decrypted credentials.
type PaymentProfileRuntime struct {
	PaymentProfile
	CredentialPlaintext string `json:"-"`
}

// Refund is an immutable record of an administrator-authorized payment reversal.
type Refund struct {
	ID             string    `json:"id"`
	PaymentOrderID string    `json:"paymentOrderId"`
	ActorUserID    *string   `json:"actorUserId"`
	TXB            Money     `json:"txb"`
	Reason         string    `json:"reason"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"createdAt"`
}

// CourtesyCredit is an immutable administrator-authorized local credit for a
// payment order that has already failed or expired without being provider-paid.
type CourtesyCredit struct {
	ID             string    `json:"id"`
	PaymentOrderID string    `json:"paymentOrderId"`
	ActorUserID    string    `json:"actorUserId"`
	TXB            Money     `json:"txb"`
	LedgerEntryID  string    `json:"ledgerEntryId"`
	Reason         string    `json:"reason"`
	CreatedAt      time.Time `json:"createdAt"`
	Replayed       bool      `json:"replayed"`
}

// TXBMoney formats integer hundredths of TXB.
func TXBMoney(minor int64) Money {
	sign := ""
	abs := uint64(minor)
	if minor < 0 {
		sign = "-"
		// Negating MinInt64 in signed arithmetic overflows. Convert its
		// magnitude in two steps so every valid ledger amount formats exactly.
		abs = uint64(-(minor + 1)) + 1
	}
	return Money{Currency: "TXB", Minor: fmt.Sprintf("%d", minor), Display: fmt.Sprintf("%s%d.%02d TXB", sign, abs/100, abs%100)}
}
