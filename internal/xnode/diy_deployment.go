package xnode

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func (s *Service) runDIYOperation(ctx context.Context, operation providerops.Operation) error {
	if providerops.Terminal(providerops.Status(operation.Receipt.Status)) {
		return nil
	}
	repo, ok := s.repo.(operationRepository)
	if !ok || s.master == nil {
		return ErrUnavailable
	}
	items, err := repo.ProviderOperationItems(ctx, operation.Receipt.ID)
	if err != nil {
		return err
	}
	if len(items) != 1 {
		return ErrInvalid
	}
	// Routing-only receipts must not wait for unrelated paid nodes. Legacy
	// receipts retain the conservative whitelist barrier during recovery.
	if items[0].TargetType == "xnode_user" && !diyWhitelistDeployment(operation.ResultJSON) {
		return s.runRoutingOperation(ctx, operation)
	}
	apply := func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		nodes, err := s.accessibleNodeIDs(ctx, operation.OwnerUserID)
		if err != nil {
			return err
		}
		ids := make([]string, 0, len(nodes))
		for id := range nodes {
			ids = append(ids, id)
		}
		slices.Sort(ids)
		if err = syncDeploymentNodes(ctx, s.master, ids); err != nil {
			return fmt.Errorf("whitelist: %w", err)
		}
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			_, settings, err := s.readDIYSettings(ctx, operation.OwnerUserID)
			if err != nil {
				return err
			}
			quota, err := s.whitelistQuota(ctx, operation.OwnerUserID)
			if err != nil {
				return err
			}
			settings.Whitelist.IPs = settings.Whitelist.IPs[:min(len(settings.Whitelist.IPs), quota)]
			pending := []string{}
			for _, id := range ids {
				if !s.master.whitelistApplied(id, operation.OwnerUserID, settings.Whitelist) {
					pending = append(pending, id)
				}
			}
			if len(pending) == 0 {
				return nil
			}
			select {
			case <-ctx.Done():
				return fmt.Errorf("whitelist confirmation nodes %v: %w", pending, ctx.Err())
			case <-ticker.C:
			}
		}
	}
	if items[0].TargetType == "xnode_user" {
		return s.runRoutingOperation(ctx, operation, apply)
	}
	if items[0].TargetType != "member_whitelist" {
		return ErrInvalid
	}
	operation, err = repo.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, s.now())
	if err != nil {
		return err
	}
	if err = apply(ctx); err != nil {
		return err
	}
	_, err = repo.CompleteXNodeProviderOperation(ctx, operation.Receipt.ID, operation.Attempts, providerops.Completion{Status: providerops.StatusSucceeded, ResultJSON: `{"whitelist":true}`}, s.now())
	return err
}

func diyWhitelistDeployment(raw string) bool {
	var scope struct {
		WhitelistChanged *bool `json:"whitelistChanged"`
	}
	if json.Unmarshal([]byte(raw), &scope) != nil || scope.WhitelistChanged == nil {
		return true
	}
	return *scope.WhitelistChanged
}
