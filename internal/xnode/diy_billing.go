package xnode

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type DIYBillingQuote struct {
	Available  bool        `json:"available"`
	Changed    bool        `json:"changed"`
	UseQuota   bool        `json:"useQuota"`
	Remaining  int         `json:"remaining"`
	Charge     model.Money `json:"charge"`
	Token      string      `json:"token"`
	ExpiresAt  int64       `json:"expiresAt"`
	PurchaseID string      `json:"purchaseId,omitempty"`
	ConfigHash string      `json:"configHash,omitempty"`
}

type diyBillingRepository interface {
	XNodeDIYBillingQuote(context.Context, string, time.Time) (DIYBillingQuote, error)
}

var ErrDIYQuoteChanged = &PolicyError{"DIY_QUOTE_CHANGED", ErrConflict}
var ErrDIYBillingUnavailable = &PolicyError{"DIY_BILLING_UNAVAILABLE", ErrInvalid}

func (s *Service) diyBillingQuote(ctx context.Context, user string) (DIYBillingQuote, error) {
	repo, ok := s.repo.(diyBillingRepository)
	if !ok {
		return DIYBillingQuote{Charge: model.TXBMoney(0)}, nil
	}
	quote, err := repo.XNodeDIYBillingQuote(ctx, user, s.now())
	if err != nil {
		return quote, err
	}
	quote.Changed = true
	quote.ExpiresAt = s.now().Add(5 * time.Minute).Unix()
	quote.Token, err = s.seal("xnode.diy.quote."+user, quote)
	return quote, err
}

func (s *Service) openDIYQuote(user, token string) (*DIYBillingQuote, error) {
	var quote DIYBillingQuote
	if token == "" || s.open("xnode.diy.quote."+user, token, &quote) != nil || quote.ExpiresAt <= s.now().Unix() {
		return nil, ErrDIYQuoteChanged
	}
	return &quote, nil
}
