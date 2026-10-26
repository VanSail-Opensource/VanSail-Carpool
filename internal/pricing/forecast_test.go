package pricing

import (
	"math"
	"testing"
)

func TestForecastMetricsExcludeUnknownAndPreserveUndefinedWAPE(t *testing.T) {
	zero, ten, twenty := 0.0, 10.0, 20.0
	for _, tc := range []struct {
		name  string
		rows  []Evaluation
		count int
		wape  *float64
	}{
		{"zero", []Evaluation{{Predicted: 5, P10: 0, P90: 8, Actual: &zero, Status: "COMPLETE"}}, 1, nil},
		{"missing_is_not_zero", []Evaluation{{Predicted: 5, Status: "MISSING"}, {Predicted: 5, Actual: &zero, Status: "CENSORED"}}, 0, nil},
		{"complete", []Evaluation{{Predicted: 12, P10: 8, P90: 15, Actual: &ten, Status: "COMPLETE"}, {Predicted: 18, P10: 15, P90: 25, Actual: &twenty, Status: "COMPLETE"}}, 2, func() *float64 { v := 4.0 / 30; return &v }()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := Evaluate(tc.rows)
			if v.Samples != tc.count {
				t.Fatal(v)
			}
			if (v.WAPE == nil) != (tc.wape == nil) {
				t.Fatal(v)
			}
			if tc.wape != nil && math.Abs(*v.WAPE-*tc.wape) > 1e-12 {
				t.Fatal(v)
			}
		})
	}
	v := Evaluate([]Evaluation{{Predicted: 12, P10: 8, P90: 15, Actual: &ten, Status: "COMPLETE"}, {Predicted: 18, P10: 15, P90: 25, Actual: &twenty, Status: "COMPLETE"}})
	if v.MAE != 2 || v.Bias != 0 || v.Coverage != 1 || v.MedianAE != 2 {
		t.Fatal(v)
	}
}

func TestEmpiricalForecastAndModelEvidenceGates(t *testing.T) {
	a, b, c, mean := EmpiricalForecast([]float64{0, 10, 20, 30, 40, math.NaN(), math.Inf(1)})
	if a != 4 || b != 20 || c != 36 || mean != 20 {
		t.Fatalf("quantiles=%v %v %v mean=%v", a, b, c, mean)
	}
	if FitResponse(make([]ResponseObservation, 199)).Validated {
		t.Fatal("insufficient evidence became calibrated")
	}
	in := OptimizationInputs{Policy: DefaultPolicy("payg", "one", 100), Current: 100, DemandGB: 10, SalesUnits: 10, CapacityGB: 100, Model: ResponseModel{Validated: false}, ForecastValidated: true}
	if _, _, err := Optimize(in); err != ModelUnvalidated {
		t.Fatal(err)
	}
	in.CommittedGB = 95
	if _, _, err := Optimize(in); err != CapacityUnsafe {
		t.Fatal("unsafe capacity was treated as a price problem", err)
	}
}
