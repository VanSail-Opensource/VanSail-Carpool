package catalog

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connectivity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type subscriptionRepositoryFixture struct {
	*catalogRepository
	disabled []string
	reads    int
	change   bool
}

func (r *subscriptionRepositoryFixture) UserDisabledSquads(context.Context, string) ([]string, error) {
	return r.disabled, nil
}
func (r *subscriptionRepositoryFixture) ActiveAndQueuedPurchases(ctx context.Context, id string, at time.Time) (*model.Purchase, *model.Purchase, error) {
	r.reads++
	if r.change && r.reads > 1 {
		return nil, nil, nil
	}
	return r.catalogRepository.ActiveAndQueuedPurchases(ctx, id, at)
}

type subscriptionRemoteFixture struct {
	*catalogRemnawave
	remoteID string
	enabled  []string
	calls    int
	err      error
}

func (r *subscriptionRemoteFixture) LoadMemberSubscription(_ context.Context, id string, enabled []string, links bool) (connectivity.MemberSubscription, error) {
	r.calls++
	r.remoteID, r.enabled = id, enabled
	result := connectivity.MemberSubscription{Hosts: []connectivity.SubscriptionHost{}, Squads: []connectivity.SubscriptionSquad{}}
	if links {
		url := "https://subscription.example/owner-key"
		result.SubscriptionURL = &url
	}
	return result, r.err
}

func TestSubscriptionAuthorizesOwnershipAndPreferences(t *testing.T) {
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	remoteID := "47"
	for _, test := range []struct {
		name      string
		active    bool
		remote    bool
		links     bool
		change    bool
		fail      bool
		wantCalls int
	}{
		{"inactive", false, true, true, false, false, 0},
		{"unprovisioned", true, false, true, false, false, 0},
		{"owner links", true, true, true, false, false, 1},
		{"credential free summary", true, true, false, false, false, 1},
		{"ownership expired in queue", true, true, true, true, false, 1},
		{"upstream failed", true, true, true, false, true, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := &subscriptionRepositoryFixture{catalogRepository: &catalogRepository{}, disabled: []string{"squad-b"}, change: test.change}
			if test.active {
				repository.active = &model.Purchase{ID: "owned-term", Status: "active", ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour), SquadUUIDs: []string{"squad-b", "squad-a"}}
			}
			remote := &subscriptionRemoteFixture{catalogRemnawave: &catalogRemnawave{}}
			if test.fail {
				remote.err = errors.New("upstream failed")
			}
			service := NewService(repository, remote, time.Minute)
			service.now = func() time.Time { return now }
			user := model.User{ID: "owner"}
			if test.remote {
				user.RemnaUserID = &remoteID
			}
			result, err := service.Subscription(context.Background(), user, test.links)
			if remote.calls != test.wantCalls {
				t.Fatalf("upstream calls=%d", remote.calls)
			}
			if test.change || test.fail {
				if err == nil || result.SubscriptionURL != nil {
					t.Fatal("failed authorization released a link")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if result.ActiveCombo != test.active {
				t.Fatal("local eligibility lost")
			}
			if remote.calls > 0 && (remote.remoteID != remoteID || !reflect.DeepEqual(remote.enabled, []string{"squad-a"})) {
				t.Fatal("another user's identity or disabled squad entered source")
			}
			if !test.links && result.SubscriptionURL != nil {
				t.Fatal("summary loaded a bearer URL")
			}
		})
	}
}
