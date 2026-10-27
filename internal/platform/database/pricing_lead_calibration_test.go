package database

import (
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
	"testing"
)

func TestOneDayCalibrationDoesNotValidateFiveLeads(t *testing.T) {
	values := map[int][]pricing.Evaluation{}
	for i := 0; i < 40; i++ {
		actual := 100.0
		if i%5 == 0 {
			actual = 120
		}
		values[1] = append(values[1], pricing.Evaluation{Predicted: 100, P10: 90, P90: 110, Actual: &actual, Status: "COMPLETE"})
	}
	confidence := model.ForecastConfidence{}
	applyPersonalCalibration(&confidence, values)
	if confidence.Calibration == "VALIDATED" || len(confidence.ByLead) != 5 || confidence.ByLead[0].Calibration != "VALIDATED" || confidence.ByLead[1].Samples != 0 {
		t.Fatal(confidence)
	}
}
