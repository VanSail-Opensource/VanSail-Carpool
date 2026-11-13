package app

import (
	"context"
	"reflect"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
)

func TestSubscriptionLoadersRejectEarlierAuthenticatedGeneration(t *testing.T) {
	for _, unified := range []bool{false, true} {
		for _, state := range []string{"revived", "purged", "blocked"} {
			t.Run(map[bool]string{false: "scoped", true: "unified"}[unified]+"/"+state, func(t *testing.T) {
				f, _, provider := newSubscriptionReadFixture(t)
				ctx := context.Background()
				statement := `UPDATE users SET account_generation=account_generation+1 WHERE id=$1`
				if state == "purged" {
					statement = `UPDATE users SET account_status='purged' WHERE id=$1`
				} else if state == "blocked" {
					statement = `UPDATE users SET bot_blocked=1 WHERE id=$1`
				}
				if _, err := f.store.DB().ExecContext(ctx, statement, f.user.ID); err != nil {
					t.Fatal(err)
				}
				load := func(user model.User) (subscription.MemberSubscription, error) {
					if unified {
						return provider.LoadUnifiedSubscription(ctx, user, true)
					}
					return provider.LoadSubscriptionForMember(ctx, user, nil, true)
				}
				result, err := load(f.user)
				if err == nil || !reflect.DeepEqual(result, subscription.MemberSubscription{}) {
					t.Fatalf("stale authentication returned subscription material: %+v, %v", result, err)
				}
				if state == "revived" {
					current, err := f.store.UserByID(ctx, f.user.ID)
					if err != nil {
						t.Fatal(err)
					}
					result, err = load(current)
					if err != nil || result.SubscriptionURL == nil || *result.SubscriptionURL == "" {
						t.Fatalf("current authenticated generation lost access: %+v, %v", result, err)
					}
				}
			})
		}
	}
}

func TestSubscriptionOwnerPostcheckDiscardsRevivedResponse(t *testing.T) {
	f, _, provider := newSubscriptionReadFixture(t)
	ctx := context.Background()
	secret := "https://panel.example/subscription/new-generation-secret"
	result, err := provider.readAuthenticatedSubscription(ctx, f.user, func() (subscription.MemberSubscription, error) {
		if _, err := f.store.DB().ExecContext(ctx, `UPDATE users SET account_generation=account_generation+1 WHERE id=$1`, f.user.ID); err != nil {
			return subscription.MemberSubscription{}, err
		}
		return subscription.MemberSubscription{SubscriptionURL: &secret, Ready: true}, nil
	})
	if err == nil || !reflect.DeepEqual(result, subscription.MemberSubscription{}) {
		t.Fatalf("owner generation changed before publication: %+v, %v", result, err)
	}
}
