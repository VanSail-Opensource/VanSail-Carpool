package pricing

import (
	"math"
	"slices"
	"time"
)

// Forecast is an original prediction vintage with a reproducible feature cutoff.
type Forecast struct {
	ID             string    `json:"forecastId"`
	EntityType     string    `json:"entityType"`
	EntityID       string    `json:"entityId"`
	Metric         string    `json:"metric"`
	GeneratedAt    time.Time `json:"generatedAt"`
	TargetFrom     time.Time `json:"targetFrom"`
	TargetUntil    time.Time `json:"targetUntil"`
	HorizonSeconds int64     `json:"horizonSeconds"`
	P10            float64   `json:"predictedP10"`
	P50            float64   `json:"predictedP50"`
	P90            float64   `json:"predictedP90"`
	Mean           float64   `json:"predictedMean"`
	ModelVersion   string    `json:"modelVersion"`
	Watermark      time.Time `json:"featureWatermark"`
	SnapshotHash   string    `json:"featureSnapshotHash"`
	CoverageBPS    int64     `json:"dataCoverageBps"`
	HealthState    string    `json:"healthState"`
	Calibration    string    `json:"calibrationStatus"`
	Actual         *float64  `json:"actual"`
	Status         string    `json:"status"`
}

// EmpiricalForecast is a transparent daily empirical distribution. It does not
// call an empirical interval calibrated until original future evaluations exist.
func EmpiricalForecast(samples []float64) (p10, p50, p90, mean float64) {
	valid := make([]float64, 0, len(samples))
	for _, v := range samples {
		if v >= 0 && !math.IsNaN(v) && !math.IsInf(v, 0) {
			valid = append(valid, v)
			mean += v
		}
	}
	if len(valid) == 0 {
		return 0, 0, 0, 0
	}
	slices.Sort(valid)
	mean /= float64(len(valid))
	q := func(p float64) float64 {
		index := p * float64(len(valid)-1)
		low := int(index)
		high := min(low+1, len(valid)-1)
		return valid[low] + (valid[high]-valid[low])*(index-float64(low))
	}
	return q(.1), q(.5), q(.9), mean
}

// Evaluation excludes missing, censored and incomplete windows from all metrics.
type Evaluation struct {
	Predicted, P10, P90 float64
	Actual              *float64
	Status              string
}

// Validation reports genuine out-of-time accuracy; undefined ratios remain null.
type Validation struct {
	Samples  int      `json:"samples"`
	Excluded int      `json:"excluded"`
	MAE      float64  `json:"mae"`
	MedianAE float64  `json:"medianAbsoluteError"`
	WAPE     *float64 `json:"wape"`
	Bias     float64  `json:"bias"`
	Coverage float64  `json:"coverage"`
	Width    float64  `json:"intervalWidth"`
	Pinball  float64  `json:"pinballLoss"`
	Brier    *float64 `json:"brierScore,omitempty"`
}

// Evaluate implements MAE, WAPE, signed bias, interval coverage and quantile loss.
func Evaluate(values []Evaluation) Validation {
	v := Validation{}
	denom, numerator := 0.0, 0.0
	errors := []float64{}
	for _, e := range values {
		if e.Status != "COMPLETE" || e.Actual == nil {
			v.Excluded++
			continue
		}
		a := *e.Actual
		if math.IsNaN(a) || math.IsInf(a, 0) {
			v.Excluded++
			continue
		}
		v.Samples++
		diff := e.Predicted - a
		abs := math.Abs(diff)
		errors = append(errors, abs)
		numerator += abs
		denom += math.Abs(a)
		v.Bias += diff
		v.Width += e.P90 - e.P10
		if a >= e.P10 && a <= e.P90 {
			v.Coverage++
		}
		for _, q := range []struct{ p, f float64 }{{.1, e.P10}, {.5, e.Predicted}, {.9, e.P90}} {
			residual := a - q.f
			v.Pinball += max(q.p*residual, (q.p-1)*residual) / 3
		}
	}
	if v.Samples == 0 {
		return v
	}
	n := float64(v.Samples)
	v.MAE = numerator / n
	v.Bias /= n
	v.Coverage /= n
	v.Width /= n
	v.Pinball /= n
	slices.Sort(errors)
	v.MedianAE = errors[len(errors)/2]
	if len(errors)%2 == 0 {
		v.MedianAE = (errors[len(errors)/2-1] + v.MedianAE) / 2
	}
	if denom > 0 {
		w := numerator / denom
		v.WAPE = &w
	}
	return v
}
