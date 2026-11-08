package hourlybilling

import (
	"context"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

const maxFundingIntents = 8192

// BuyHourlyExtra receives authenticated live demand only from the existing
// master credit worker. A member cannot supply its observed timestamp or bytes.
func (s *Service) BuyHourlyExtra(ctx context.Context, request xnode.HourlyExtraRequest) error {
	now := s.now().UTC()
	if request.OwnerID == "" || request.AccountGeneration < 0 || request.PendingBytes <= 0 ||
		request.PendingObservedAt.IsZero() || request.PendingObservedAt.After(now) || now.Sub(request.PendingObservedAt) > 4*time.Second || request.DayUTC != now.Format(time.DateOnly) {
		return database.ErrHourlyDemandStale
	}
	scope := fundScope{request.OwnerID, request.AccountGeneration, request.DayUTC}
	intent, err := s.fundingIntent(ctx, scope, request)
	if err != nil {
		return err
	}
	_, err = s.store.FundHourlyExtraGB(ctx, intent.input, intent.snapshot, now)
	s.mu.Lock()
	current, found := s.funds[scope]
	if found && current.id == intent.id {
		if err == nil || definitiveFundingFailure(err) {
			delete(s.funds, scope)
		} else {
			current.running = false
			s.funds[scope] = current
		}
	}
	s.mu.Unlock()
	return err
}

func (s *Service) fundingIntent(ctx context.Context, scope fundScope, request xnode.HourlyExtraRequest) (fundIntent, error) {
	s.mu.Lock()
	if existing, found := s.funds[scope]; found {
		if existing.running {
			s.mu.Unlock()
			return fundIntent{}, wait("HOURLY_EXTRA_PENDING", ErrUnavailable)
		}
		existing.running = true
		s.funds[scope] = existing
		s.mu.Unlock()
		return existing, nil
	}
	// A prior day's receipt remains durable; expiring its RAM retry identity
	// changes no funds or credit and its SQL midnight fence forbids late debit.
	for key, value := range s.funds {
		if key.day < request.DayUTC && !value.running {
			delete(s.funds, key)
		}
	}
	if len(s.funds) >= maxFundingIntents {
		s.mu.Unlock()
		return fundIntent{}, ErrUnavailable
	}
	s.mu.Unlock()
	snapshot, err := s.policy.HourlyPolicy(ctx)
	if err != nil {
		return fundIntent{}, err
	}
	id, err := ids.New()
	if err != nil {
		return fundIntent{}, err
	}
	intent := fundIntent{id: id, snapshot: snapshot, running: true,
		input: database.HourlyExtraFundingInput{ReceiptID: id, OwnerID: scope.owner, AccountGeneration: scope.generation,
			DayUTC: scope.day, ExpectedPolicyVersion: snapshot.Version, PendingBytes: uint64(request.PendingBytes), PendingObservedAt: request.PendingObservedAt}}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, found := s.funds[scope]; found || len(s.funds) >= maxFundingIntents {
		return fundIntent{}, wait("HOURLY_EXTRA_PENDING", ErrUnavailable)
	}
	s.funds[scope] = intent
	return intent, nil
}

func definitiveFundingFailure(err error) bool {
	return businessFailure(err) || errors.Is(err, database.ErrHourlyDemandStale) || errors.Is(err, database.ErrHourlyNotExhausted) ||
		errors.Is(err, database.ErrHourlyExtraDisabled) || errors.Is(err, model.ErrHourlyCreditExpired)
}
