package xnode

import (
	"context"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

type ddnsApplyRepository interface {
	ddnsChangeRepository
	ApplyXNodeDDNSChange(context.Context, DDNSChangeState, DDNSChangeInput, time.Time) error
	AdvanceXNodeDDNSChange(context.Context, DDNSChangeState, string, []string) error
}

func (s *Service) runDDNSOperation(ctx context.Context, operation providerops.Operation) error {
	if providerops.Terminal(providerops.Status(operation.Receipt.Status)) {
		return nil
	}
	repo, ok := s.repo.(ddnsApplyRepository)
	if !ok || s.master == nil {
		return ErrUnavailable
	}
	states, err := repo.XNodeDDNSChanges(ctx)
	if err != nil {
		return err
	}
	var state *DDNSChangeState
	for index := range states {
		if states[index].OperationID == operation.Receipt.ID {
			state = &states[index]
			break
		}
	}
	if state == nil {
		return s.completeDDNSOperation(ctx, operation, true)
	}
	if state.Phase == "staging" {
		in, err := s.prepareDDNSChange(ctx, state.NodeID, state.AgentID, state.Change)
		if errors.Is(err, ErrConflict) {
			in, err = nil, nil
		}
		if err != nil {
			return err
		}
		if in == nil {
			if err = repo.AdvanceXNodeDDNSChange(ctx, *state, "obsolete", state.PreviousIPs); err != nil {
				return err
			}
			if err = s.master.refreshDDNSProtection(ctx); err != nil {
				return err
			}
			return s.completeDDNSOperation(ctx, operation, true)
		}
		ready, err := s.master.stageDDNSProtection(ctx)
		if err != nil {
			return err
		}
		if !ready {
			return outbox.ErrBusy
		}
		if err = repo.ApplyXNodeDDNSChange(ctx, *state, *in, s.now().UTC()); err != nil {
			return err
		}
		state.Phase = "routes"
		if err = s.master.refreshDDNSProtection(ctx); err != nil {
			return err
		}
		s.master.NotifyFirewallChanged()
	}
	if state.Phase == "routes" {
		ready, err := s.refreshDDNSRoutes(ctx, *state, operation.ActorUserID)
		if err != nil {
			return err
		}
		if !ready {
			return ErrUnavailable
		}
		references, err := s.ddnsAddressReferences(ctx)
		if err != nil {
			return err
		}
		previous := []string{}
		// Reload: apply may have retained a concurrently edited old host.
		states, err = repo.XNodeDDNSChanges(ctx)
		if err != nil {
			return err
		}
		for _, current := range states {
			if current.OperationID == state.OperationID {
				for _, ip := range current.PreviousIPs {
					if references[ip] {
						previous = append(previous, ip)
					}
				}
			}
		}
		if err = repo.AdvanceXNodeDDNSChange(ctx, *state, "settled", previous); err != nil {
			return err
		}
		if err = s.master.refreshDDNSProtection(ctx); err != nil {
			return err
		}
		s.master.NotifyFirewallChanged()
	}
	return s.completeDDNSOperation(ctx, operation, state.Phase == "obsolete")
}

func (s *Service) completeDDNSOperation(ctx context.Context, operation providerops.Operation, obsolete bool) error {
	repo, ok := s.repo.(operationRepository)
	if !ok {
		return ErrUnavailable
	}
	// DDNS never debits a wallet or creates a member configuration quote.
	operation, err := repo.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, s.now().UTC())
	if err != nil {
		return err
	}
	result := `{"ddns":true,"redistributed":true}`
	if obsolete {
		result = `{"ddns":true,"obsolete":true}`
	}
	_, err = repo.CompleteXNodeProviderOperation(ctx, operation.Receipt.ID, operation.Attempts, providerops.Completion{Status: providerops.StatusSucceeded, ResultJSON: result}, s.now().UTC())
	return err
}
