package catalog

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connectivity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type nativeSubscriptionRemote struct{ *subscriptionRemoteFixture }

func (r *nativeSubscriptionRemote) LoadSubscriptionForMember(ctx context.Context, user model.User, enabled []string, links bool) (connectivity.MemberSubscription, error) {
	return r.LoadMemberSubscription(ctx, "native:"+user.ID, enabled, links)
}

func TestNativeSubscriptionWithoutLegacyIdentityRetainsOwnershipFence(t *testing.T) {
	for _, changed := range []bool{false, true} {
		now := time.Now().UTC()
		repo := &subscriptionRepositoryFixture{catalogRepository: &catalogRepository{}, change: changed, disabled: []string{"disabled"}}
		repo.active = &model.Purchase{ID: "purchase", UserID: "member", Status: "active", ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour), SquadUUIDs: []string{"enabled", "disabled"}}
		remote := &nativeSubscriptionRemote{subscriptionRemoteFixture: &subscriptionRemoteFixture{catalogProxy: &catalogProxy{}}}
		service := NewService(repo, remote, time.Minute)
		service.now = func() time.Time { return now }
		result, err := service.Subscription(context.Background(), model.User{ID: "member"}, true)
		if remote.remoteID != "native:member" || remote.calls != 1 || len(remote.enabled) != 1 || remote.enabled[0] != "enabled" {
			t.Fatal("native subscription lost ownership or preferences", remote)
		}
		if changed {
			if err == nil || result.SubscriptionURL != nil {
				t.Fatal("expired term released credentials", result, err)
			}
			continue
		}
		if err != nil || result.SubscriptionURL == nil || !result.ActiveCombo {
			t.Fatal("native-only subscription unavailable", result, err)
		}
	}
}
