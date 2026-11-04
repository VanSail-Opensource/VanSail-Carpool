package database

import (
	"context"
	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/squadprofile"
	"testing"
)

func TestComboFeatureOrderOmissionAndClear(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	input := ComboInput{Name: "Core", PriceTXBMinor: 100, ValidityDays: 30, TrafficLimitBytes: 1000, ResetStrategy: "MONTH_ROLLING", Active: true, Features: squadprofile.Features{{Icon: "rocket-launch", Title: " First ", Detail: " Detail "}, {Icon: "lock-key", Title: "Second"}}}
	combo, err := s.SaveCombo(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if len(combo.Features) != 2 || combo.Features[0].Title != "First" || combo.Features[0].Detail != "Detail" {
		t.Fatal(combo.Features)
	}
	input.ID = combo.ID
	input.Features = nil
	combo, err = s.SaveCombo(ctx, input)
	if err != nil || len(combo.Features) != 2 {
		t.Fatal(combo.Features, err)
	}
	input.Features = squadprofile.Features{{Icon: "unsafe", Title: "Invalid"}}
	if _, err = s.SaveCombo(ctx, input); err == nil {
		t.Fatal("unsafe icon accepted")
	}
	input.Features = squadprofile.Features{}
	combo, err = s.SaveCombo(ctx, input)
	if err != nil || len(combo.Features) != 0 {
		t.Fatal(combo.Features, err)
	}
	raw, _ := json.Marshal(combo.Features)
	if string(raw) != "[]" {
		t.Fatal(string(raw))
	}
}
