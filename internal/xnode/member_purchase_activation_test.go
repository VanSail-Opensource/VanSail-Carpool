package xnode

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type memberActivationRepository struct {
	commerceRepository
	user            model.User
	missingPurchase bool
	saves           int
}

func (r *memberActivationRepository) XNodeIdentityByUser(context.Context, string) (Identity, error) {
	return r.identity, nil
}
func (r *memberActivationRepository) UserByID(context.Context, string) (model.User, error) {
	return r.user, nil
}
func (r *memberActivationRepository) DesiredEntitlement(context.Context, string, time.Time) (*model.Purchase, error) {
	if r.missingPurchase {
		return nil, nil
	}
	return &r.purchase, nil
}
func (r *memberActivationRepository) ListPurchases(context.Context, string) ([]model.Purchase, error) {
	if r.missingPurchase {
		return nil, nil
	}
	return []model.Purchase{r.purchase}, nil
}
func (r *memberActivationRepository) SaveXNodePeriod(_ context.Context, period Period) error {
	r.saves++
	r.period = period
	return nil
}

func TestMemberActivationCannotTargetMatchingSuccessorOrEmptyDesiredState(t *testing.T) {
	for _, scenario := range []string{"successor", "missing", "foreign identity", "generation changed", "revision changed", "same-clock start", "same-clock combo", "same-clock squads", "same term"} {
		t.Run(scenario, func(t *testing.T) {
			now := time.Now().UTC()
			user := model.User{ID: "owner", TelegramID: 4242, OnboardingState: "complete", AccountStatus: "active", AccountGeneration: 2}
			purchase := model.Purchase{ID: "target", UserID: user.ID, Status: "active", ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour), UpdatedAt: now, TrafficLimitBytes: 100, ResetStrategy: "DAY"}
			repository := &memberActivationRepository{commerceRepository: commerceRepository{
				identity: Identity{ProxyUserID: 42, UserID: user.ID, CurrentPeriodID: "retained", CredentialsCipher: "fixture", SubscriptionCipher: "fixture", SubscriptionHash: make([]byte, 32)},
				period:   Period{ID: "retained", ProxyUserID: 42, PurchaseID: purchase.ID, Status: "open", UsedBytes: 77}, purchase: purchase}, user: user}
			switch scenario {
			case "successor":
				repository.purchase.ID = "successor"
				repository.period.PurchaseID = "successor"
			case "missing":
				repository.missingPurchase = true
			case "foreign identity":
				repository.identity.UserID = "another"
			case "generation changed":
				repository.user.AccountGeneration++
			case "revision changed":
				repository.purchase.UpdatedAt = now.Add(time.Second)
			case "same-clock start":
				repository.purchase.ValidFrom = now.Add(-time.Minute)
			case "same-clock combo":
				repository.purchase.ComboID = "edited-combo"
			case "same-clock squads":
				repository.purchase.SquadUUIDs = []string{"edited-squad"}
			}
			service := &Service{repo: repository, now: func() time.Time { return now }}
			err := service.ApplyMemberPurchase(context.Background(), user, purchase)
			if scenario == "same term" {
				if err != nil || repository.saves != 1 || repository.period.LimitBytes != 100 {
					t.Fatal(err, repository.period)
				}
			} else if !errors.Is(err, ErrConflict) || repository.saves != 0 {
				t.Fatal("obsolete activation touched current period", err, repository.saves)
			}
			if repository.period.ID != "retained" || repository.period.UsedBytes != 77 || repository.period.Status != "open" {
				t.Fatal("activation reset or closed retained accounting", repository.period)
			}
		})
	}
}
