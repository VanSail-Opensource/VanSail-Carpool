package xnode

import (
	"context"
	"encoding/json"
	"math"
	"slices"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func speedtestPairTarget(pair RouteSpeedtestPair) string {
	// Only public IDs are encoded; this canonical form is also the durable result
	// metadata, independent of later route edits or deleted endpoint inventories.
	value, _ := json.Marshal(pair)
	return string(value)
}

func validSpeedtestMeasurement(value SpeedtestMeasurement) bool {
	if len(value.Latencies) != 3 {
		return false
	}
	for _, number := range append(slices.Clone(value.Latencies), value.DownloadMbps, value.UploadMbps) {
		if math.IsNaN(number) || math.IsInf(number, 0) || number < 0 || number > 1e12 {
			return false
		}
	}
	return true
}

func routeSpeedtestEstimate(pairs []RouteSpeedtestPairResult) *RouteSpeedtestEstimate {
	if len(pairs) == 0 {
		return nil
	}
	result := &RouteSpeedtestEstimate{DownloadMbps: math.Inf(1), UploadMbps: math.Inf(1)}
	for _, pair := range pairs {
		if pair.Status != string(providerops.StatusSucceeded) || pair.Measurement == nil || !validSpeedtestMeasurement(*pair.Measurement) {
			return nil
		}
		latencies := slices.Clone(pair.Measurement.Latencies)
		slices.Sort(latencies)
		result.LatencyMillis += latencies[len(latencies)/2]
		result.DownloadMbps = min(result.DownloadMbps, pair.Measurement.DownloadMbps)
		result.UploadMbps = min(result.UploadMbps, pair.Measurement.UploadMbps)
	}
	return result
}

func (s *Service) OwnedRouteSpeedtest(ctx context.Context, user, id string) (RouteSpeedtestResult, error) {
	result := RouteSpeedtestResult{Pairs: []RouteSpeedtestPairResult{}}
	repo, ok := s.repo.(routeSpeedtestRepository)
	if !ok {
		return result, ErrUnavailable
	}
	op, err := repo.ProviderOperationByID(ctx, id)
	if isMissing(err) {
		return result, ErrNotFound
	}
	if err != nil {
		return result, err
	}
	if op.Receipt.Kind != RouteSpeedtestOperation || op.OwnerUserID != user {
		return result, ErrNotFound
	}
	items, err := repo.ProviderOperationItems(ctx, id)
	if err != nil {
		return result, err
	}
	result.Receipt = op.Receipt
	for _, item := range items {
		pair := RouteSpeedtestPairResult{Status: string(item.Status), ErrorCode: item.ErrorCode}
		if providerops.Terminal(providerops.Status(op.Receipt.Status)) && item.Status == providerops.StatusQueued {
			pair.Status, pair.ErrorCode = "compensated", "XNODE_SPEEDTEST_NOT_RUN"
		}
		if json.Unmarshal([]byte(item.TargetID), &pair.RouteSpeedtestPair) != nil || pair.SourceNodeID == "" || pair.TargetEndpointID == "" {
			return result, ErrInvalid
		}
		if item.Status == providerops.StatusSucceeded {
			var measurement SpeedtestMeasurement
			if json.Unmarshal([]byte(item.ResultJSON), &measurement) != nil || !validSpeedtestMeasurement(measurement) {
				return result, ErrUnavailable
			}
			pair.Measurement = &measurement
		}
		result.Pairs = append(result.Pairs, pair)
	}
	result.Estimate = routeSpeedtestEstimate(result.Pairs)
	return result, nil
}
