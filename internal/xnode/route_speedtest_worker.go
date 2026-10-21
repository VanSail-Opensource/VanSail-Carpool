package xnode

import (
	"context"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	payload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func (s *Service) handleRouteSpeedtest(ctx context.Context, op providerops.Operation, job model.OutboxJob) error {
	sealed, err := payload.TargetID(job, "sealedTarget")
	if err != nil {
		return err
	}
	var doc routeSpeedtestDocument
	if err = s.open("xnode.route-speedtest."+op.ActorUserID+"."+op.IdempotencyKey, sealed, &doc); err != nil {
		return err
	}
	if len(doc.Pairs) < 1 || len(doc.Pairs) > 64 {
		return ErrInvalid
	}
	return s.master.deferOperation(job, time.Duration(len(doc.Pairs))*2*time.Minute+time.Minute, func(ctx context.Context) error { return s.runRouteSpeedtest(ctx, op, doc) })
}

func (s *Service) runRouteSpeedtest(ctx context.Context, op providerops.Operation, doc routeSpeedtestDocument) error {
	repo, ok := s.repo.(routeSpeedtestRepository)
	if !ok {
		return ErrUnavailable
	}
	var err error
	op, err = repo.BeginProviderOperationAttempt(ctx, op.Receipt.ID, s.now())
	if err != nil {
		return err
	}
	items, err := repo.ProviderOperationItems(ctx, op.Receipt.ID)
	if err != nil {
		return err
	}
	if len(items) != len(doc.Pairs) {
		return ErrInvalid
	}
	status, code := providerops.StatusSucceeded, ""
	for index, item := range items {
		pair := doc.Pairs[index]
		if item.Key != fmt.Sprintf("%02d", index) || item.TargetID != speedtestPairTarget(pair) || item.TargetType != "xnode_speedtest" {
			return ErrInvalid
		}
		if item.Status == providerops.StatusProcessing {
			// Durable intent can mean the agent consumed traffic. A restart/lost
			// result must never replay this command, even after agent dedup expires.
			item, err = repo.CompleteProviderOperationItem(ctx, op.Receipt.ID, item.Key, providerops.Completion{Status: providerops.StatusPendingReview, ErrorCode: "XNODE_COMMAND_OUTCOME_UNKNOWN", ResultJSON: "{}"}, s.now())
			if err != nil {
				return err
			}
		}
		if item.Status == providerops.StatusQueued {
			result, runErr := s.runRouteSpeedtestPair(ctx, op, doc, item.Key, pair)
			if runErr != nil {
				return runErr
			}
			item = result
		}
		if err = s.flushSpeedtestPair(ctx, doc.PeriodID, op.Receipt.ID, item.Key); err != nil {
			return err
		}
		if item.Status != providerops.StatusSucceeded {
			status, code = providerops.StatusFailed, item.ErrorCode
			if index > 0 {
				status = providerops.StatusPartial
			}
			if item.Status == providerops.StatusPendingReview {
				status = providerops.StatusPendingReview
			}
			break
		}
	}
	_, err = repo.CompleteXNodeProviderOperation(ctx, op.Receipt.ID, op.Attempts, providerops.Completion{Status: status, ErrorCode: code, ResultJSON: "{}"}, s.now())
	return err
}

func (s *Service) flushSpeedtestPair(ctx context.Context, period, operation, key string) error {
	repo, ok := s.probeRepository()
	if !ok {
		return ErrUnavailable
	}
	probes, err := repo.XNodePeriodProbes(ctx, period)
	if err != nil {
		return err
	}
	for _, p := range probes {
		if p.OperationID == operation && p.PairKey == key {
			return s.flushMeteredProbe(ctx, p)
		}
	}
	return nil
}
