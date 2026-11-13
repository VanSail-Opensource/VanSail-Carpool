package xnode

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestStableSubscriptionReadRetriesOnlyBenignContention(t *testing.T) {
	for _, change := range []string{"proof", "proof-before-begin", "epoch", "local", "boundary", "token", "owner"} {
		t.Run(change, func(t *testing.T) {
			now := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
			repo := &subscriptionReadRepository{
				identity: Identity{ProxyUserID: 7, UserID: "owner", CredentialsCipher: "credentials", SubscriptionCipher: "token"},
				owner:    model.User{ID: "owner", AccountStatus: "active"},
				revision: 1, boundary: now.Add(time.Second),
			}
			service := NewService(repo, nil, nil, nil, nil)
			service.now = func() time.Time { return now }
			captured := repo.identity
			if change == "proof-before-begin" {
				repo.identity.FirstSyncedAt = &now
			}
			calls := 0
			result, err := ReadSubscription(context.Background(), service, captured, func(identity Identity) (int, error) {
				calls++
				if calls == 1 {
					switch change {
					case "proof":
						repo.identity.FirstSyncedAt = &now
					case "epoch":
						repo.revision++
					case "local":
						service.localCompileRevision.Add(1)
					case "boundary":
						now = repo.boundary
						repo.boundary = time.Time{}
					case "token":
						repo.identity.SubscriptionCipher = "replacement"
					case "owner":
						repo.owner.AccountGeneration++
					}
				} else if (change == "proof" || change == "proof-before-begin") && identity.FirstSyncedAt == nil {
					t.Fatal("retry did not reload confirmed readiness")
				}
				return calls, nil
			})
			if change == "token" || change == "owner" {
				if !errors.Is(err, ErrConflict) || calls != 1 || result != 0 {
					t.Fatal("authority change was followed", calls, result, err)
				}
			} else if err != nil || calls != 2 || result != 2 {
				t.Fatal("benign contention did not repeat the full read", calls, result, err)
			}
		})
	}
}

type subscriptionRetryOwnerRepository struct {
	*subscriptionReadRepository
	versionReads int
}

func (r *subscriptionRetryOwnerRepository) XNodeCompileVersion(ctx context.Context, now time.Time) (int64, time.Time, error) {
	r.versionReads++
	if r.versionReads == 3 {
		// Commit an owner lifecycle change after Check read the old owner but
		// before its final epoch read. Retrying must retain that old owner anchor.
		r.owner.AccountGeneration++
		r.revision++
	}
	return r.subscriptionReadRepository.XNodeCompileVersion(ctx, now)
}

func TestStableSubscriptionReadRetainsOwnerAcrossEpochRetry(t *testing.T) {
	base := &subscriptionReadRepository{
		identity: Identity{ProxyUserID: 7, UserID: "owner"},
		owner:    model.User{ID: "owner", AccountStatus: "active"},
	}
	repo := &subscriptionRetryOwnerRepository{subscriptionReadRepository: base}
	service := NewService(repo, nil, nil, nil, nil)
	calls := 0
	result, err := ReadSubscription(context.Background(), service, base.identity, func(Identity) (int, error) {
		calls++
		return calls, nil
	})
	if !errors.Is(err, ErrConflict) || result != 0 || calls != 1 {
		t.Fatal("retry crossed owner generation", result, calls, err)
	}
}

func TestStableSubscriptionReadBoundsContinuousContention(t *testing.T) {
	repo := &subscriptionReadRepository{
		identity: Identity{ProxyUserID: 7, UserID: "owner"},
		owner:    model.User{ID: "owner", AccountStatus: "active"},
	}
	service := NewService(repo, nil, nil, nil, nil)
	calls := 0
	result, err := ReadSubscription(context.Background(), service, repo.identity, func(Identity) (int, error) {
		calls++
		repo.revision++
		return calls, nil
	})
	if !errors.Is(err, ErrConflict) || result != 0 || calls != 3 {
		t.Fatal("continuous contention was unbounded or published", result, calls, err)
	}
}
