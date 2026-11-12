package xnode

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
)

func TestMemberSubscriptionRotationUsesOwnerAndExactGeneration(t *testing.T) {
	foreign := "999"
	for _, pointer := range []*string{nil, &foreign} {
		ctx := context.Background()
		at := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
		user := model.User{ID: "owner", ProxyUserID: pointer, AccountGeneration: 3, AccountStatus: "active", OnboardingState: "complete"}
		repo := &memberRotationRepository{subscriptionReadRepository: &subscriptionReadRepository{
			identity: Identity{ProxyUserID: 42, UserID: user.ID, CredentialsCipher: "credentials", FirstSyncedAt: &at},
			owner:    user, revision: 1,
		}}
		vault, err := secret.NewVault(make([]byte, 32))
		if err != nil {
			t.Fatal(err)
		}
		public, err := url.Parse("https://panel.example")
		if err != nil {
			t.Fatal(err)
		}
		service := NewService(repo, nil, vault, public, nil)
		if err := service.newSubscription(&repo.identity); err != nil {
			t.Fatal(err)
		}
		old := repo.identity.SubscriptionCipher
		link, err := service.RevokeSubscriptionForMember(ctx, user)
		if err != nil || !strings.HasPrefix(link, "https://panel.example/subscription/") || repo.lookedUp != user.ID ||
			repo.rotations != 1 || repo.generation != user.AccountGeneration || repo.identity.SubscriptionCipher == old ||
			repo.identity.ProxyUserID != 42 || repo.identity.CredentialsCipher != "credentials" {
			t.Fatalf("rotation lost owner, generation or retained material: link=%q rotations=%d generation=%d err=%v", link, repo.rotations, repo.generation, err)
		}
		// A lifecycle change between lookup and mutation must reach the atomic
		// generation-aware repository path, never the raw identity rotation.
		repo.owner.AccountGeneration++
		old = repo.identity.SubscriptionCipher
		if _, err = service.RevokeSubscriptionForMember(ctx, user); !errors.Is(err, ErrConflict) || repo.identity.SubscriptionCipher != old || repo.rotations != 1 {
			t.Fatal("retired generation rotated current credentials", err)
		}
	}
}

func TestMemberSubscriptionRotationRejectsForeignAndServiceIdentity(t *testing.T) {
	for _, identity := range []Identity{{UserID: "other", ProxyUserID: 42}, {UserID: "owner", ServiceName: "standalone", ProxyUserID: 42}} {
		repo := &memberRotationRepository{subscriptionReadRepository: &subscriptionReadRepository{identity: identity}}
		service := NewService(repo, nil, nil, nil, nil)
		if _, err := service.RevokeSubscriptionForMember(context.Background(), model.User{ID: "owner"}); !errors.Is(err, ErrConflict) || repo.rotations != 0 {
			t.Fatal("foreign or standalone identity reached mutation", err)
		}
	}
}

type memberRotationRepository struct {
	*subscriptionReadRepository
	lookedUp   string
	rotations  int
	generation int64
}

func (r *memberRotationRepository) XNodeIdentityByUser(_ context.Context, owner string) (Identity, error) {
	r.lookedUp = owner
	return r.identity, nil
}

func (r *memberRotationRepository) RotateXNodeMemberSubscription(_ context.Context, expected Identity, generation int64, hash []byte, cipher string) (Identity, error) {
	r.generation = generation
	if expected.UserID != r.owner.ID || generation != r.owner.AccountGeneration {
		return Identity{}, ErrConflict
	}
	r.rotations++
	r.identity.SubscriptionHash = append([]byte(nil), hash...)
	r.identity.SubscriptionCipher = cipher
	return r.identity, nil
}
