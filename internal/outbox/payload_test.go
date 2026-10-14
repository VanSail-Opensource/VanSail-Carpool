package outbox

import (
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestTargetID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, payload, want string
	}{
		{name: "legacy string fields", payload: `{"drawId":"draw-1","revision":"2"}`, want: "draw-1"},
		{name: "numeric revision", payload: `{"drawId":"draw-1","revision":2}`, want: "draw-1"},
		{name: "mixed metadata", payload: `{"drawId":" draw-1 ","revision":2,"enabled":true,"snapshot":{"seats":[1,2]},"optional":null}`, want: "draw-1"},
		{name: "missing target", payload: `{"revision":2}`},
		{name: "blank target", payload: `{"drawId":" \t "}`},
		{name: "null target", payload: `{"drawId":null}`},
		{name: "numeric target", payload: `{"drawId":123,"revision":2}`},
		{name: "boolean target", payload: `{"drawId":true}`},
		{name: "object target", payload: `{"drawId":{"id":"draw-1"}}`},
		{name: "array target", payload: `{"drawId":["draw-1"]}`},
		{name: "malformed JSON", payload: `{"drawId":"draw-1",`},
		{name: "array payload", payload: `[{"drawId":"draw-1"}]`},
		{name: "null payload", payload: `null`},
	}
	for _, kind := range []string{"draw_telegram_update", "draw_raffle_settle"} {
		t.Run(kind, func(t *testing.T) {
			for _, test := range tests {
				t.Run(test.name, func(t *testing.T) {
					got, err := TargetID(model.OutboxJob{Kind: kind, Payload: test.payload}, "drawId")
					if got != test.want || (err != nil) != (test.want == "") {
						t.Fatalf("TargetID() = (%q, %v), want %q with error=%t", got, err, test.want, test.want == "")
					}
				})
			}
		})
	}
}

func TestDecodePaymentSuccessAnnouncementProviderNameCompatibility(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		json string
		want string
	}{
		{name: "legacy payload", json: `{"orderId":"order-1","provider":"ezpay","channel":"alipay","txbMinor":100,"payableAmount":"1.00","payableCurrency":"CNY","username":"@ada"}`},
		{name: "provider snapshot", json: `{"orderId":"order-1","provider":"ezpay","providerName":"  Main EZPay  ","channel":"alipay","txbMinor":100,"payableAmount":"1.00","payableCurrency":"CNY","username":"@ada"}`, want: "Main EZPay"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			payload, err := DecodePaymentSuccessAnnouncement(model.OutboxJob{Kind: PaymentSuccessAnnouncementKind, Payload: test.json})
			if err != nil {
				t.Fatalf("DecodePaymentSuccessAnnouncement(): %v", err)
			}
			if payload.ProviderName != test.want {
				t.Errorf("ProviderName = %q, want %q", payload.ProviderName, test.want)
			}
		})
	}
}
