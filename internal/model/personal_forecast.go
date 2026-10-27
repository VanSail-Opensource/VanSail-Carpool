package model

import "time"

type ForecastDay struct {
	Date           string  `json:"date"`
	ActualBytes    *string `json:"actualBytes"`
	PredictedBytes *string `json:"predictedBytes"`
	LowerBytes     *string `json:"lowerBytes"`
	UpperBytes     *string `json:"upperBytes"`
	Status         string  `json:"status"`
}

type ForecastConfidence struct {
	ByLead                    []ForecastLeadConfidence `json:"byLead,omitempty"`
	Calibration               string                   `json:"calibration"`
	Samples                   int                      `json:"samples"`
	ObservedDays              int                      `json:"observedDays"`
	ObservedCoverageBPS       int64                    `json:"observedCoverageBps"`
	NominalIntervalBPS        int64                    `json:"nominalIntervalBps"`
	EvaluatedIntervalCoverage *float64                 `json:"evaluatedIntervalCoverage"`
}

// ForecastLeadConfidence evaluates original vintages for one calendar lead only.
type ForecastLeadConfidence struct {
	LeadDays         int      `json:"leadDays"`
	Samples          int      `json:"samples"`
	Calibration      string   `json:"calibration"`
	IntervalCoverage *float64 `json:"intervalCoverage"`
	WAPE             *float64 `json:"wape"`
}

type PersonalForecast struct {
	RefreshPending  bool               `json:"refreshPending"`
	Days            []ForecastDay      `json:"days"`
	GeneratedAt     *time.Time         `json:"generatedAt"`
	ModelVersion    string             `json:"modelVersion"`
	Timezone        string             `json:"timezone"`
	Stale           bool               `json:"stale"`
	Status          string             `json:"status"`
	Confidence      ForecastConfidence `json:"confidence"`
	RenewalDecision *PricingDecision   `json:"renewalDecision"`
}
