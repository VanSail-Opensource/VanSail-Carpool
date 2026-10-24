package xnode

import (
	"context"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestMemberAdmissionSurvivesLongCommandsAndKeepsFirewallReserve(t *testing.T) {
	m := &Master{}
	ordinary := model.OutboxJob{Kind: "provider_operation"}
	for range 3 {
		if !m.reserveOperation(ordinary, false) {
			t.Fatal("ordinary capacity reduced")
		}
	}
	if m.reserveOperation(ordinary, false) {
		t.Fatal("ordinary capacity became unbounded")
	}
	for range 4 {
		if !m.reserveOperation(ordinary, true) {
			t.Fatal("independent member was blocked by other operations")
		}
	}
	if m.reserveOperation(ordinary, true) {
		t.Fatal("member capacity was blocked or became unbounded")
	}
	urgent := model.OutboxJob{Kind: FirewallJob, Payload: `{"bank":"dynamic"}`}
	if !m.reserveOperation(urgent, false) || m.reserveOperation(urgent, false) {
		t.Fatal("firewall reserve lost or exceeded")
	}
}

func TestDeferredPanicReturnsFailureAndCancellationPropagates(t *testing.T) {
	if err := runDeferredOperation(context.Background(), func(context.Context) error { panic("private") }); err == nil || err.Error() != "deferred node operation panicked" {
		t.Fatal("panic escaped or leaked details", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := runDeferredOperation(ctx, func(ctx context.Context) error { return ctx.Err() }); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation lost", err)
	}
}

func TestDIYRoutingOnlyReceiptSkipsUnrelatedWhitelistDeployment(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      bool
	}{
		{"routing only", `{"diySave":{"routingChanged":true},"whitelistChanged":false}`, false},
		{"combined change", `{"whitelistChanged":true}`, true},
		{"legacy receipt", `{"diySave":{"routingChanged":true}}`, true},
		{"missing metadata", `{}`, true},
		{"malformed metadata", `invalid`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := diyWhitelistDeployment(tc.raw); got != tc.want {
				t.Fatal("unsafe deployment scope", got)
			}
		})
	}
}
