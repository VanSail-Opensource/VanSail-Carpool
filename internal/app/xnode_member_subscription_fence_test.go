package app

import (
	"context"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

type subscriptionReadRepository struct {
	*database.Store
	afterEndpoints func(context.Context) error
	mutations      int
}

func (r *subscriptionReadRepository) XNodeEndpoints(ctx context.Context) ([]xnode.Endpoint, error) {
	endpoints, err := r.Store.XNodeEndpoints(ctx)
	if err != nil {
		return nil, err
	}
	if change := r.afterEndpoints; change != nil {
		r.afterEndpoints = nil
		r.mutations++
		if err := change(ctx); err != nil {
			return nil, err
		}
	}
	return endpoints, nil
}

func newSubscriptionReadFixture(t *testing.T) (nativePurchaseFixture, *subscriptionReadRepository, proxyProvider) {
	t.Helper()
	f := newNativePurchaseFixture(t)
	ctx := context.Background()
	// Seed historical application proof for the fixture's current credentials.
	if _, err := f.store.DB().ExecContext(ctx, `UPDATE xnode_identities SET first_synced_at=$1 WHERE user_id=$2`,
		time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano), f.user.ID); err != nil {
		t.Fatal(err)
	}
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	public, err := url.Parse("https://panel.example")
	if err != nil {
		t.Fatal(err)
	}
	repository := &subscriptionReadRepository{Store: f.store}
	native := xnode.NewService(repository, nil, vault, public, nil)
	return f, repository, proxyProvider{native: native, store: f.store}
}

func TestMemberSubscriptionDiscardsViewChangedDuringInventory(t *testing.T) {
	for _, change := range []string{"restriction", "subscription rotation"} {
		for _, links := range []bool{false, true} {
			t.Run(change+map[bool]string{false: "/metadata", true: "/links"}[links], func(t *testing.T) {
				f, repository, provider := newSubscriptionReadFixture(t)
				ctx := context.Background()
				baseline, err := provider.LoadUnifiedSubscription(ctx, f.user, links)
				if err != nil || !baseline.Ready || links && (baseline.SubscriptionURL == nil || *baseline.SubscriptionURL == "") {
					t.Fatalf("fixture subscription = %+v, %v", baseline, err)
				}
				identity, err := provider.native.IdentityForUser(ctx, f.user.ID)
				if err != nil {
					t.Fatal(err)
				}
				repository.afterEndpoints = func(ctx context.Context) error {
					if change == "restriction" {
						return repository.Store.SetXNodeAccessDisabled(ctx, identity.ProxyUserID, true)
					}
					_, err := f.provider.native.RevokeSubscription(ctx, f.remoteID)
					return err
				}
				view, err := provider.LoadUnifiedSubscription(ctx, f.user, links)
				if err == nil || repository.mutations != 1 || !reflect.DeepEqual(view, subscription.MemberSubscription{}) {
					t.Fatalf("changed subscription escaped: view=%+v, err=%v, mutations=%d", view, err, repository.mutations)
				}
			})
		}
	}
}

func TestMemberSubscriptionURLFailureDiscardsProjection(t *testing.T) {
	f, _, provider := newSubscriptionReadFixture(t)
	ctx := context.Background()
	if _, err := f.store.DB().ExecContext(ctx, `UPDATE xnode_identities SET subscription_cipher='invalid-ciphertext' WHERE user_id=$1`, f.user.ID); err != nil {
		t.Fatal(err)
	}
	view, err := provider.LoadUnifiedSubscription(ctx, f.user, true)
	if err == nil || !reflect.DeepEqual(view, subscription.MemberSubscription{}) {
		t.Fatalf("failed URL returned a partial subscription: view=%+v, err=%v", view, err)
	}
}

func TestMemberSubscriptionRetriesFirstSyncProofGainWithOriginalLinkIntent(t *testing.T) {
	f, repository, provider := newSubscriptionReadFixture(t)
	ctx := context.Background()
	if _, err := f.store.DB().ExecContext(ctx, `UPDATE xnode_identities SET first_synced_at=NULL WHERE user_id=$1`, f.user.ID); err != nil {
		t.Fatal(err)
	}
	repository.afterEndpoints = func(ctx context.Context) error {
		_, err := f.store.DB().ExecContext(ctx, `UPDATE xnode_identities SET first_synced_at=$1 WHERE user_id=$2`,
			time.Now().UTC().Format(time.RFC3339Nano), f.user.ID)
		return err
	}
	view, err := provider.LoadUnifiedSubscription(ctx, f.user, true)
	if err != nil || repository.mutations != 1 || !view.Ready || view.SubscriptionURL == nil || *view.SubscriptionURL == "" {
		t.Fatalf("first-sync recovery lost requested links: view=%+v err=%v mutations=%d", view, err, repository.mutations)
	}
}
