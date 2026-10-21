package xnode

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"time"
)

func (s *Service) runRouteSpeedtestPair(ctx context.Context, op providerops.Operation, doc routeSpeedtestDocument, key string, pair RouteSpeedtestPair) (providerops.Item, error) {
	repo := s.repo.(routeSpeedtestRepository)
	current, err := s.routeSpeedtestPlan(ctx, op.OwnerUserID, doc.Request)
	if err == nil && (current.PeriodID != doc.PeriodID || !sameSpeedtestPairs(current.Pairs, doc.Pairs)) {
		err = ErrConflict
	}
	if err != nil {
		if errors.Is(err, ErrUnavailable) {
			return providerops.Item{}, err
		}
		return s.finishUnstartedSpeedtestPair(ctx, repo, op, key)
	}
	period, err := s.repo.XNodePeriod(ctx, doc.PeriodID)
	if err != nil {
		return providerops.Item{}, err
	}
	endpoints, err := s.Endpoints(ctx)
	if err != nil {
		return providerops.Item{}, err
	}
	var target Endpoint
	for _, endpoint := range endpoints {
		if endpoint.ID == pair.TargetEndpointID && endpoint.Enabled {
			target = endpoint
			break
		}
	}
	if target.ID == "" {
		return s.finishUnstartedSpeedtestPair(ctx, repo, op, key)
	}
	identity, err := s.prepareMeteredProbe(ctx, op.Receipt.ID, key, period, target)
	if errors.Is(err, ErrConflict) {
		return s.finishUnstartedSpeedtestPair(ctx, repo, op, key)
	}
	if err != nil {
		return providerops.Item{}, err
	}
	if err = s.master.Sync(ctx, target.NodeID); err != nil {
		return providerops.Item{}, err
	}
	if err = s.master.WaitApplied(ctx, target.NodeID); err != nil {
		return providerops.Item{}, err
	}
	proxy, err := s.endpointProxy(ctx, identity, target)
	if err != nil {
		return providerops.Item{}, err
	}
	outbound, err := proxy.Outbound()
	if err != nil {
		return providerops.Item{}, err
	}
	global, err := s.Global(ctx)
	if err != nil {
		return providerops.Item{}, err
	}
	if _, err = repo.BeginProviderOperationItemAttempt(ctx, op.Receipt.ID, key, s.now()); err != nil {
		return providerops.Item{}, err
	}
	child, cancel := context.WithTimeout(ctx, 90*time.Second)
	raw, commandErr := s.master.Command(child, pair.SourceNodeID, op.Receipt.ID+"-"+key, "speedtest", map[string]any{"peer_outbound": outbound, "download_url": global.SpeedtestDownloadURL, "upload_url": global.SpeedtestUploadURL, "max_bytes": 32 << 20, "timeout_seconds": 60})
	cancel()
	completion := speedtestCompletion(raw, commandErr)
	finish, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer stop()
	return repo.CompleteProviderOperationItem(finish, op.Receipt.ID, key, completion, s.now())
}

func (s *Service) finishUnstartedSpeedtestPair(ctx context.Context, repo routeSpeedtestRepository, op providerops.Operation, key string) (providerops.Item, error) {
	if _, err := repo.BeginProviderOperationItemAttempt(ctx, op.Receipt.ID, key, s.now()); err != nil {
		return providerops.Item{}, err
	}
	return repo.CompleteProviderOperationItem(ctx, op.Receipt.ID, key, providerops.Completion{Status: providerops.StatusFailed, ErrorCode: "XNODE_SPEEDTEST_ACCESS_CHANGED", ResultJSON: "{}"}, s.now())
}

func sameSpeedtestPairs(a, b []RouteSpeedtestPair) bool {
	if len(a) != len(b) {
		return false
	}
	for index := range a {
		if a[index] != b[index] {
			return false
		}
	}
	return true
}

func speedtestCompletion(raw json.RawMessage, err error) providerops.Completion {
	completion := providerops.Completion{Status: providerops.StatusPendingReview, ErrorCode: "XNODE_COMMAND_OUTCOME_UNKNOWN", ResultJSON: "{}"}
	if err != nil {
		var command *CommandError
		if errors.As(err, &command) && command.Outcome != CommandUncertain {
			completion.Status = providerops.StatusFailed
			completion.ErrorCode = "XNODE_COMMAND_FAILED"
		}
		return completion
	}
	var measurement SpeedtestMeasurement
	if json.Unmarshal(raw, &measurement) != nil || !validSpeedtestMeasurement(measurement) {
		return completion
	}
	value, marshalErr := json.Marshal(measurement)
	if marshalErr != nil {
		return completion
	}
	return providerops.Completion{Status: providerops.StatusSucceeded, ResultJSON: string(value)}
}
