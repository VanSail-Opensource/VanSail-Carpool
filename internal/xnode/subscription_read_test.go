package xnode

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
)

type subscriptionReadRepository struct {
	Repository
	identity Identity
	owner    model.User
	revision int64
	boundary time.Time
	blocked  bool
	failure  error
}

func (r *subscriptionReadRepository) XNodeCompileVersion(context.Context, time.Time) (int64, time.Time, error) {
	return r.revision, r.boundary, r.failure
}

func (r *subscriptionReadRepository) XNodeIdentity(context.Context, int64) (Identity, error) {
	return r.identity, nil
}

func (r *subscriptionReadRepository) UserByID(context.Context, string) (model.User, error) {
	return r.owner, nil
}

func (r *subscriptionReadRepository) XNodeUserRestricted(context.Context, string) (bool, error) {
	return r.blocked, nil
}

func TestSubscriptionReadRejectsAuthorityChangesBeforePublication(t *testing.T) {
	for _, name := range []string{"credentials", "token", "token-hash", "proof", "period", "disabled", "generation", "restriction", "onboarding", "epoch", "local-epoch", "boundary"} {
		t.Run(name, func(t *testing.T) {
			now := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
			repo := &subscriptionReadRepository{
				identity: Identity{ProxyUserID: 7, UserID: "owner", CredentialsCipher: "credentials", SubscriptionCipher: "token"},
				owner:    model.User{ID: "owner", AccountStatus: "active", OnboardingState: "complete"},
				revision: 1, boundary: now.Add(time.Hour),
			}
			service := NewService(repo, nil, nil, nil, nil)
			service.now = func() time.Time { return now }
			fence, err := service.BeginSubscriptionRead(context.Background(), repo.identity)
			if err != nil {
				t.Fatal(err)
			}
			if err = service.CheckSubscriptionRead(context.Background(), fence); err != nil {
				t.Fatal("unchanged read rejected", err)
			}
			raw, err := json.Marshal(fence)
			if err != nil || string(raw) != "{}" {
				t.Fatal("private read authority escaped", string(raw), err)
			}
			switch name {
			case "credentials":
				repo.identity.CredentialsCipher = "replacement"
			case "token":
				repo.identity.SubscriptionCipher = "replacement"
			case "token-hash":
				repo.identity.SubscriptionHash = []byte("replacement")
			case "proof":
				repo.identity.FirstSyncedAt = &now
			case "period":
				repo.identity.CurrentPeriodID = "replacement"
			case "disabled":
				repo.identity.AccessDisabled = true
			case "generation":
				repo.owner.AccountGeneration++
			case "restriction":
				repo.blocked = true
			case "onboarding":
				repo.owner.OnboardingState = "pending"
			case "epoch":
				repo.revision++
			case "local-epoch":
				service.localCompileRevision.Add(1)
			case "boundary":
				now = repo.boundary
			}
			if err = service.CheckSubscriptionRead(context.Background(), fence); !errors.Is(err, ErrConflict) {
				t.Fatal("changed authority was published", name, err)
			}
		})
	}
}

func TestSubscriptionReadRequiresCurrentIdentityAndEpoch(t *testing.T) {
	ctx := context.Background()
	repo := &subscriptionReadRepository{
		identity: Identity{ProxyUserID: 7, UserID: "owner", SubscriptionCipher: "current"},
		owner:    model.User{ID: "owner"},
	}
	service := NewService(repo, nil, nil, nil, nil)
	stale := repo.identity
	stale.SubscriptionCipher = "revoked"
	if _, err := service.BeginSubscriptionRead(ctx, stale); !errors.Is(err, ErrConflict) {
		t.Fatal("stale caller identity accepted", err)
	}
	fence, err := service.BeginSubscriptionRead(ctx, repo.identity)
	if err != nil {
		t.Fatal(err)
	}
	unavailable := errors.New("epoch read failed")
	repo.failure = unavailable
	if err = service.CheckSubscriptionRead(ctx, fence); !errors.Is(err, unavailable) {
		t.Fatal("repository failure was hidden", err)
	}
	other := NewService(repo, nil, nil, nil, nil)
	if err = other.CheckSubscriptionRead(ctx, fence); !errors.Is(err, ErrConflict) {
		t.Fatal("foreign service fence accepted", err)
	}
	if err = service.CheckSubscriptionRead(ctx, SubscriptionReadFence{}); !errors.Is(err, ErrConflict) {
		t.Fatal("zero fence accepted", err)
	}
	unsupported := NewService(deniedResponseRepository{}, nil, nil, nil, nil)
	if _, err = unsupported.BeginSubscriptionRead(ctx, repo.identity); !errors.Is(err, ErrUnavailable) {
		t.Fatal("unfenced repository allowed publication", err)
	}
}

func TestStandaloneSubscriptionReadExpiresDuringRendering(t *testing.T) {
	now := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	repo := &subscriptionReadRepository{identity: Identity{ProxyUserID: 7, ServiceName: "service"}}
	credentials := Credentials{ServiceAccess: &ServiceAccess{
		StartedAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Second), Strategy: "NO_RESET",
	}}
	repo.identity.CredentialsCipher, err = sealCredentials(vault, repo.identity, credentials)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(repo, nil, vault, nil, nil)
	service.now = func() time.Time { return now }
	fence, err := service.BeginSubscriptionRead(context.Background(), repo.identity)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	if err = service.CheckSubscriptionRead(context.Background(), fence); !errors.Is(err, ErrConflict) {
		t.Fatal("standalone expiry crossed during rendering", err)
	}
}
