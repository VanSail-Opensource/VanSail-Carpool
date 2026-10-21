package xnode

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/routingmatch"
)

const RouteSpeedtestOperation = "xnode_route_speedtest"

type RouteSpeedtestRequest struct {
	Revision        int64                    `json:"revision"`
	Start           string                   `json:"start"`
	StartEndpointID string                   `json:"startEndpointId"`
	Destination     routingmatch.Destination `json:"destination"`
}

type RouteSpeedtestPair struct {
	SourceNodeID     string `json:"sourceNodeId"`
	TargetEndpointID string `json:"targetEndpointId"`
}

type routeSpeedtestDocument struct {
	Request  RouteSpeedtestRequest `json:"request"`
	PeriodID string                `json:"periodId"`
	Pairs    []RouteSpeedtestPair  `json:"pairs"`
}

// SpeedtestMeasurement never drives billing. Only normal traffic journal
// samples attributed through MeteredProbe update the originating paid period.
type SpeedtestMeasurement struct {
	Latencies    []float64 `json:"request_latency_millis"`
	DownloadMbps float64   `json:"download_mbps"`
	UploadMbps   float64   `json:"upload_mbps"`
}

type RouteSpeedtestPairResult struct {
	RouteSpeedtestPair
	Status      string                `json:"status"`
	ErrorCode   string                `json:"errorCode,omitempty"`
	Measurement *SpeedtestMeasurement `json:"measurement,omitempty"`
}

type RouteSpeedtestResult struct {
	Receipt  model.OperationReceipt     `json:"receipt"`
	Pairs    []RouteSpeedtestPairResult `json:"pairs"`
	Estimate *RouteSpeedtestEstimate    `json:"estimate,omitempty"`
}

type RouteSpeedtestEstimate struct {
	LatencyMillis float64 `json:"latencyMillis"`
	DownloadMbps  float64 `json:"downloadMbps"`
	UploadMbps    float64 `json:"uploadMbps"`
}

type routeSpeedtestRepository interface {
	operationRepository
	probeRepository
	ReplayXNodeRouteSpeedtest(context.Context, string, string, string) (model.OperationReceipt, bool, error)
	ProviderOperationByID(context.Context, string) (providerops.Operation, error)
	BeginProviderOperationItemAttempt(context.Context, string, string, time.Time) (providerops.Item, error)
	CompleteProviderOperationItem(context.Context, string, string, providerops.Completion, time.Time) (providerops.Item, error)
}
