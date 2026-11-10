package xnode

import (
	"context"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
)

func TestFirewallDeliveryWaitKeepsIdempotentOfflineDeliveryRetryable(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		err  error
		wait bool
	}{
		{name: "undelivered", err: &CommandError{Outcome: CommandUndelivered, Cause: ErrUnavailable}, wait: true},
		{name: "uncertain", err: &CommandError{Outcome: CommandUncertain, Cause: ErrUnavailable}, wait: true},
		{name: "rejected", err: &CommandError{Outcome: CommandRejected, Cause: ErrUnavailable}, wait: false},
		{name: "uncertain deadline", err: &CommandError{Outcome: CommandUncertain, Cause: context.DeadlineExceeded}, wait: true},
		{name: "agent failure", err: &CommandError{Outcome: CommandFailed, Cause: ErrUnavailable}, wait: false},
		{name: "repository error", err: errors.New("load firewall records"), wait: false},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := firewallDeliveryWait(test.err)
			var wait *outbox.WaitError
			if test.wait != errors.As(got, &wait) {
				t.Fatalf("firewallDeliveryWait(%v) = %v", test.err, got)
			}
		})
	}
}
