package database

import (
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"testing"
)

func TestCleanupPairClassificationNeverTreatsUnknownAsPurgeEvidence(t *testing.T) {
	for _, tc := range []struct {
		action, chat                 string
		prior, reachable, qualifying bool
	}{
		{"reachable", "unknown", false, true, false},
		{"blocked", "private", false, false, true},
		{"blocked", "unknown", true, false, false},
		{"unknown", "unreachable", true, false, false},
		{"deactivated", "private", true, false, false},
		{"deactivated", "deactivated", false, false, true},
		{"unreachable", "unreachable", false, false, false},
		{"unreachable", "unreachable", true, false, true},
		{"unreachable", "private", true, false, false},
	} {
		t.Run(tc.action+"/"+tc.chat, func(t *testing.T) {
			reachable, qualifying, valid := classifyCleanupObservation(model.AccountCleanupObservation{SendAction: tc.action, GetChat: tc.chat}, tc.prior)
			if !valid || reachable != tc.reachable || qualifying != tc.qualifying {
				t.Fatal(reachable, qualifying, valid)
			}
		})
	}
	if _, _, valid := classifyCleanupObservation(model.AccountCleanupObservation{SendAction: "timeout", GetChat: "private"}, false); valid {
		t.Fatal("unvalidated category accepted")
	}
}
