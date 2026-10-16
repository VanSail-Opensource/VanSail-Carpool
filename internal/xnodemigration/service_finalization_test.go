package xnodemigration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestFinalStandaloneFenceIncludesTrafficResetAndAccessTerms(t *testing.T) {
	service, repo, _ := finalizationFixture(t)
	repo.identities = []xnode.Identity{{ServiceName: "standalone", LegacyUserID: "929"}}
	access := xnode.ServiceAccess{LimitBytes: 1000, Strategy: "MONTH_ROLLING", StartedAt: time.Now().UTC().Add(-time.Hour), ExpiresAt: time.Now().UTC().Add(time.Hour), GroupIDs: []string{"existing-group"}}
	before := Account{LegacyID: 929, ServiceName: "standalone", UsedBytes: 10, LifetimeBytes: 20, ServiceAccess: &access}
	snapshot := Snapshot{Accounts: []Account{before}}
	if err := service.verifyFinalAccounts(context.Background(), snapshot, []Account{before}); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"usage", "lifetime", "reset", "quota", "expiry", "groups", "disabled", "missing-policy"} {
		t.Run(field, func(t *testing.T) {
			after, policy := before, access
			after.ServiceAccess = &policy
			switch field {
			case "usage":
				after.UsedBytes++
			case "lifetime":
				after.LifetimeBytes++
			case "reset":
				at := time.Now()
				after.LastResetAt = &at
			case "quota":
				policy.LimitBytes++
			case "expiry":
				policy.ExpiresAt = policy.ExpiresAt.Add(time.Hour)
			case "groups":
				policy.GroupIDs = []string{"different-group"}
			case "disabled":
				policy.Disabled = true
			case "missing-policy":
				after.ServiceAccess = nil
			}
			if err := service.verifyFinalAccounts(context.Background(), snapshot, []Account{after}); !errors.Is(err, xnode.ErrConflict) {
				t.Fatalf("changed standalone %s was accepted: %v", field, err)
			}
		})
	}
}
