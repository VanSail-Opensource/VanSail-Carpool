package xnode

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestRouteSpeedtestEstimatesUseSumOfMediansAndDirectionalMinima(t *testing.T) {
	pairs := []RouteSpeedtestPairResult{{Status: "succeeded", Measurement: &SpeedtestMeasurement{Latencies: []float64{3, 100, 4}, DownloadMbps: 20, UploadMbps: 8}}, {Status: "succeeded", Measurement: &SpeedtestMeasurement{Latencies: []float64{11, 9, 10}, DownloadMbps: 12, UploadMbps: 15}}}
	value := routeSpeedtestEstimate(pairs)
	if value == nil || value.LatencyMillis != 14 || value.DownloadMbps != 12 || value.UploadMbps != 8 {
		t.Fatal("estimate", value)
	}
	pairs[1].Status = "pending_review"
	if routeSpeedtestEstimate(pairs) != nil {
		t.Fatal("uncertain pair produced an estimate")
	}
}

func TestSpeedtestResultsNeverDriveBillingAndUncertainOutcomesStayUncertain(t *testing.T) {
	value := speedtestCompletion(json.RawMessage(`{"request_latency_millis":[1,2,3],"download_mbps":12,"upload_mbps":8,"download_bytes":999999,"peer_outbound":{"password":"secret"}}`), nil)
	if value.Status != providerops.StatusSucceeded || strings.Contains(value.ResultJSON, "bytes") || strings.Contains(value.ResultJSON, "secret") {
		t.Fatal("unsafe result", value)
	}
	for _, err := range []error{&CommandError{CommandUncertain, ErrUnavailable}, context.Canceled} {
		if value := speedtestCompletion(nil, err); value.Status != providerops.StatusPendingReview {
			t.Fatal("uncertain result became retryable", value)
		}
	}
	if value := speedtestCompletion(json.RawMessage(`{"request_latency_millis":[],"download_mbps":0,"upload_mbps":0}`), nil); value.Status != providerops.StatusPendingReview {
		t.Fatal("malformed success trusted", value)
	}
}

func TestMeteredProbeAdmissionExpiresAndNeverFollowsSuccessorPeriods(t *testing.T) {
	now := time.Now()
	p := MeteredProbe{PeriodID: "origin", ValidUntil: now.Add(time.Minute)}
	period := Period{ID: "origin", Status: "open", LimitBytes: 10, UsedBytes: 9}
	owner := Identity{CurrentPeriodID: "origin"}
	if !meteredProbePeriodEligible(p, period, owner, now) {
		t.Fatal("valid origin denied")
	}
	for _, kind := range []string{"expired", "revoked", "quota", "reset", "renewal", "refund", "disabled"} {
		t.Run(kind, func(t *testing.T) {
			probe, term, user := p, period, owner
			switch kind {
			case "expired":
				probe.ValidUntil = now
			case "revoked":
				probe.Revoked = true
			case "quota":
				term.UsedBytes = 10
			case "reset", "renewal":
				user.CurrentPeriodID = "successor"
			case "refund":
				term.Status = "closing"
			case "disabled":
				user.AccessDisabled = true
			}
			if meteredProbePeriodEligible(probe, term, user, now) {
				t.Fatal("probe retained stale paid access")
			}
		})
	}
}

type recoveredSpeedtestRepository struct {
	Repository
	routeSpeedtestRepository
	op        providerops.Operation
	items     []providerops.Item
	completed providerops.Completion
}

func (r *recoveredSpeedtestRepository) BeginProviderOperationAttempt(context.Context, string, time.Time) (providerops.Operation, error) {
	r.op.Attempts++
	return r.op, nil
}
func (r *recoveredSpeedtestRepository) ProviderOperationByID(context.Context, string) (providerops.Operation, error) {
	return r.op, nil
}
func (r *recoveredSpeedtestRepository) ProviderOperationItems(context.Context, string) ([]providerops.Item, error) {
	return r.items, nil
}
func (r *recoveredSpeedtestRepository) XNodePeriodProbes(context.Context, string) ([]MeteredProbe, error) {
	return nil, nil
}
func (r *recoveredSpeedtestRepository) CompleteProviderOperationItem(_ context.Context, _, key string, c providerops.Completion, _ time.Time) (providerops.Item, error) {
	for i := range r.items {
		if r.items[i].Key == key {
			r.items[i].Status = c.Status
			r.items[i].ErrorCode = c.ErrorCode
			r.items[i].ResultJSON = c.ResultJSON
			return r.items[i], nil
		}
	}
	return providerops.Item{}, ErrNotFound
}
func (r *recoveredSpeedtestRepository) CompleteXNodeProviderOperation(_ context.Context, _ string, _ int, c providerops.Completion, _ time.Time) (providerops.Operation, error) {
	r.completed = c
	return r.op, nil
}

func TestRecoveredRouteSpeedtestRetainsCompletedPairsAndNeverResendsUncertainPair(t *testing.T) {
	pairs := []RouteSpeedtestPair{{SourceNodeID: "a", TargetEndpointID: "b-host"}, {SourceNodeID: "b", TargetEndpointID: "c-host"}}
	completed := `{"request_latency_millis":[1,2,3],"download_mbps":8,"upload_mbps":4}`
	r := &recoveredSpeedtestRepository{op: providerops.Operation{Receipt: model.OperationReceipt{ID: "operation", Kind: RouteSpeedtestOperation, Status: "processing"}, OwnerUserID: "member"}, items: []providerops.Item{{Key: "00", TargetType: "xnode_speedtest", TargetID: speedtestPairTarget(pairs[0]), Status: providerops.StatusSucceeded, ResultJSON: completed}, {Key: "01", TargetType: "xnode_speedtest", TargetID: speedtestPairTarget(pairs[1]), Status: providerops.StatusProcessing}}}
	s := &Service{repo: r, now: time.Now} // No master: any attempted traffic replay fails.
	if err := s.runRouteSpeedtest(context.Background(), r.op, routeSpeedtestDocument{PeriodID: "origin", Pairs: pairs}); err != nil {
		t.Fatal(err)
	}
	if r.completed.Status != providerops.StatusPendingReview || r.items[0].ResultJSON != completed || r.items[0].Status != providerops.StatusSucceeded || r.items[1].Status != providerops.StatusPendingReview {
		t.Fatal("recovery lost certainty", r.completed, r.items)
	}
	if _, err := s.OwnedRouteSpeedtest(context.Background(), "other", r.op.Receipt.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("owner result escaped", err)
	}
}
