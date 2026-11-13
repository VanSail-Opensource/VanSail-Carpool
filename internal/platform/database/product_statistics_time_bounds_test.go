package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestProductStatisticsRetainExactActiveInterval(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	from := time.Date(2026, 11, 13, 17, 0, 0, 0, time.UTC)
	until := from.Add(time.Second + time.Nanosecond)
	squad := saveTestSquad(t, store, "11111111-1111-4111-8111-111111111111", 0, true)
	combo := saveTestCombo(t, store, "Active interval", 100, 30, squad.ID)
	member, purchase := createAdminWorkflowPurchase(t, store, 93_201, combo, from)
	if _, err := store.SaveXNodeIdentity(ctx, xnode.Identity{UserID: member.ID, CredentialsCipher: "opaque"}); err != nil {
		t.Fatal(err)
	}
	usageBreakdownExec(t, store, `UPDATE purchases SET valid_from=$1,valid_until=$2,auto_renew_enabled=1 WHERE id=$3`, stamp(from), stamp(until), purchase.ID)
	for _, item := range []struct {
		name   string
		at     time.Time
		active bool
	}{
		{"before start", from.Add(-time.Nanosecond), false},
		{"start", from, true},
		{"fractional second", from.Add(time.Nanosecond), true},
		{"last nanosecond", until.Add(-time.Nanosecond), true},
		{"end", until, false},
	} {
		t.Run(item.name, func(t *testing.T) {
			value, err := store.ProductDatabaseStatistics(ctx, item.at)
			if err != nil {
				t.Fatal(err)
			}
			members, err := store.ActiveMemberUsageForStatistics(ctx, item.at)
			if err != nil {
				t.Fatal(err)
			}
			if item.active {
				if shareValue(value.SubscriptionStates, "auto_renew") != 1 || len(value.ComboShares) != 1 ||
					len(value.ComboBySquad) != 1 || len(value.SquadByCombo) != 1 || len(members) != 1 || members[0].Purchase.ID != purchase.ID {
					t.Fatalf("active purchase disappeared at %v: %+v, %+v", item.at, value, members)
				}
			} else if shareValue(value.SubscriptionStates, "no_active") != 1 || len(value.ComboShares) != 0 ||
				len(value.ComboBySquad) != 0 || len(value.SquadByCombo) != 0 || len(members) != 0 {
				t.Fatalf("inactive purchase included at %v: %+v, %+v", item.at, value, members)
			}
		})
	}
}
