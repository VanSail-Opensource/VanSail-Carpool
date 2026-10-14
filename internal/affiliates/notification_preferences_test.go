package affiliates

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"testing"
)

type affiliateDeliveryPolicy struct {
	allowed bool
	err     error
}

func (p *affiliateDeliveryPolicy) TelegramNotificationAllowed(context.Context, int64, string) (bool, error) {
	return p.allowed, p.err
}

type affiliateDeliverySender struct{ calls int }

func (s *affiliateDeliverySender) SendMarkdownV2Message(context.Context, int64, int64, string) error {
	s.calls++
	return nil
}

func TestAffiliateDeliveryChecksLatestPreferences(t *testing.T) {
	sender := &affiliateDeliverySender{}
	policy := &affiliateDeliveryPolicy{}
	w := NewNotificationWorker(sender, policy)
	payload, err := json.Marshal(jobpayload.AffiliateSuccess{SettlementID: "settlement", ChatID: 42, Locale: "en", InviteeName: "Lin", SettledAt: "2026-10-07T00:00:00Z", CommissionMinor: 100, TierName: "Member"})
	if err != nil {
		t.Fatal(err)
	}
	job := model.OutboxJob{Kind: jobpayload.AffiliateSuccessKind, Payload: string(payload)}
	if err := w.HandleOutbox(context.Background(), job); err != nil || sender.calls != 0 {
		t.Fatalf("suppression=%v calls=%d", err, sender.calls)
	}
	policy.err = errors.New("database unavailable")
	if err := w.HandleOutbox(context.Background(), job); err == nil || sender.calls != 0 {
		t.Fatalf("lookup retry=%v calls=%d", err, sender.calls)
	}
	policy.err = nil
	policy.allowed = true
	if err := w.HandleOutbox(context.Background(), job); err != nil || sender.calls != 1 {
		t.Fatalf("delivery=%v calls=%d", err, sender.calls)
	}
}
